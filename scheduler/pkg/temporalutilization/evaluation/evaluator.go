package evaluation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/clock"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	schedv1alpha1 "gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	tu "gitee.com/openeuler/paws/scheduler/pkg/temporalutilization"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/events"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/utils"
	"github.com/go-logr/logr"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	kcache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var log = logf.Log.WithName("usage_evaluator")

type UsageEvaluator struct {
	client           client.Client
	reconcilerScheme *runtime.Scheme
	// loopContexts keep tracks of the individual coroutine running/pending evaluation
	loopContexts         *sync.Map
	globalHTTPTimeout    time.Duration
	recorder             record.EventRecorder
	promClient           *PromClient
	evaluationResolution time.Duration

	// a synchronization queue for all the periodic evaluation
	// to avoid too many spin off goroutines.
	mu          *sync.RWMutex
	clock       clock.Clock
	evaluationQ *cache.Heap
}

// custom Comp Fn for queue sorting
func CompFn(obj1, obj2 interface{}) bool {
	q1, _ := obj1.(*tu.QueuedUsageTemplate)
	q2, _ := obj2.(*tu.QueuedUsageTemplate)
	return q1.NextEvaluationTime.Before(q2.NextEvaluationTime)
}

func NewUsageEvaluator(c client.Client, reconcilerScheme *runtime.Scheme, evaluationResolution, globalHTTPTimeout time.Duration, recorder record.EventRecorder, promAddress string) (*UsageEvaluator, error) {
	pClient, err := NewPromClient(promAddress)
	if err != nil {
		log.Error(err, "unable to create prometheus client", "PromAddress", promAddress)
		return nil, err
	}
	return &UsageEvaluator{
		client:               c,
		reconcilerScheme:     reconcilerScheme,
		loopContexts:         &sync.Map{},
		globalHTTPTimeout:    globalHTTPTimeout,
		recorder:             recorder,
		promClient:           pClient,
		evaluationResolution: evaluationResolution,
		clock:                clock.RealClock{},
		evaluationQ:          kcache.NewHeap(kcache.MetaNamespaceKeyFunc, CompFn),
		mu:                   &sync.RWMutex{},
	}, nil
}

func (ue *UsageEvaluator) DeleteUsageTemplateEvaluation(ctx context.Context, object interface{}) error {
	ut, ok := object.(*schedv1alpha1.UsageTemplate)
	if !ok {
		err := fmt.Errorf("unknown object type %v", object)
		log.Error(err, "error deleting usage template", "object", object)
		return err
	}

	key, err := kcache.MetaNamespaceKeyFunc(ut)
	if err != nil {
		log.Error(err, "unable to obtain namespacekey", "UsageTemplate", ut)
		return err
	}

	stored, ok := ue.loopContexts.Load(key)
	if !ok {
		log.V(1).Info("UsageTemplate loop context was not found in controller cache", "key", key)
	} else {
		cancel, ok := stored.(context.CancelFunc)
		if ok {
			// cancel the evaluation loop for getting the usage template
			cancel()
		}
		ue.loopContexts.Delete(key)
		ue.recorder.Event(ut, corev1.EventTypeNormal, events.EvaluationStopped, "Stopped evaluation loop")
	}

	return nil
}

func (ue *UsageEvaluator) HandleUsageTemplate(ctx context.Context, ut *schedv1alpha1.UsageTemplate) error {
	key, err := kcache.MetaNamespaceKeyFunc(ut)
	if err != nil {
		return err
	}

	// NOTE: this context is passed down to the periodic evaluation
	// to enable cancelling in the future.
	ctx, cancel := context.WithCancel(ctx)

	// cancel the previously started evaluation for the same ut object if exists
	value, loaded := ue.loopContexts.LoadOrStore(key, cancel)
	if loaded {
		oldCancel, ok := value.(context.CancelFunc)
		if ok {
			oldCancel()
		}
		ue.loopContexts.Store(key, cancel)
	} else {
		ue.recorder.Event(ut, corev1.EventTypeNormal, events.EvaluationStarted, "started periodic evaluation")
	}

	// avoid global object shared with deep copy
	go ue.startEvaluation(ctx, ut.DeepCopy())

	return nil
}

func (ue *UsageEvaluator) startEvaluation(ctx context.Context, ut *schedv1alpha1.UsageTemplate) {

	hours := ut.Spec.EvaluatePeriodHours
	if hours == nil {
		*hours = schedv1alpha1.DefaultEvaluationPeriodHours
	}

	log.V(3).Info("adding usage template to queue", "UsageTemplate", GetNamespacedName(ut))

	qUt := &tu.QueuedUsageTemplate{
		UsageTemplate:      ut,
		Counts:             0,
		NextEvaluationTime: ue.clock.Now(),
		Context:            ctx,
	}
	ue.addToEvaluationQueue(qUt)
}

func (ue *UsageEvaluator) addToEvaluationQueue(qUT *tu.QueuedUsageTemplate) {
	select {
	case <-qUT.Context.Done():
		{
			log.V(3).Info("context done, not adding to evaluation Q", "usageTemplate", GetNamespacedName(qUT.UsageTemplate))
			return
		}
	default:
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()
	if err := ue.evaluationQ.Add(qUT); err != nil {
		log.Error(err, "unable to add to queue", "QueuedUsageTemplate", GetNamespacedName(qUT.UsageTemplate))
	}
}

func (ue *UsageEvaluator) evaluateResources(ctx context.Context, logger logr.Logger, ut *schedv1alpha1.UsageTemplate) {

	// 2. evaluate each resource for the selected pods
	for _, resourceType := range ut.Spec.Resources {
		ue.evaluateResource(ctx, resourceType, ut)
	}
}

func (ue *UsageEvaluator) evaluateResource(ctx context.Context, resourceType string, ut *schedv1alpha1.UsageTemplate) {
	query, err := ue.buildUsageQuery(ut.Spec.Filters, resourceType, ut.Spec.JoinFilters, ut.Spec.JoinLabels)
	if err != nil {
		log.Error(err, "unable to build query", "Resource", resourceType, "Filters", ut.Spec.Filters)
		utils.UpdateReadyConditions(ctx, ue.client, log, ut, metav1.ConditionFalse, "Unable to build Prometheus Query", "BuildUsageQueryError")
		return
	}

	end := time.Now().UTC()

	evaluationDays := v1alpha1.DefaultEvaluationWindowDays
	if ut.Spec.EvaluationWindowDays != nil {
		evaluationDays = int(*ut.Spec.EvaluationWindowDays)
	}

	// inverse
	start := end.AddDate(0, 0, -evaluationDays)

	metricTS, err := ue.promClient.FetchQueryRange(ctx, query, ue.globalHTTPTimeout, start, end, ue.evaluationResolution, log)
	if err != nil {
		log.Error(err, "failed querying prometheus", "Query", query)
		utils.UpdateReadyConditions(ctx, ue.client, log, ut, metav1.ConditionFalse, "Unable to fetch from Prometheus", "FetchQueryError")
		return
	}

	// aggregate into per hour samples for a histogram
	// TODO: how much overhead here to rebuild this everytime
	h, err := ue.buildHistogram(metricTS)
	if err != nil {
		log.Error(err, "failed to build datetime decaying histogram", "Resource", resourceType, "Metric", metricTS, "Query", query)
		utils.UpdateReadyConditions(ctx, ue.client, log, ut, metav1.ConditionFalse, "Unable to build histogram", "BuildHistogramError")
		return
	}

	// take the percentile value from it
	err = ue.estimateHourUsage(ctx, ut, h, resourceType, v1alpha1.SupportedResourceMetricScalingFactor[resourceType])
	if err != nil {
		log.Error(err, "failed to estimate hourly usage", "Resource", resourceType)
		utils.UpdateReadyConditions(ctx, ue.client, log, ut, metav1.ConditionFalse, "Unable to estimate hourly usage", "EstimateHourlyUsageError")
		return
	}
	log.V(3).Info("successfully evaluated usage template", "usageTemplate", GetNamespacedName(ut), "Query", query)
}

// Because cAdvisor by default only assign the 'whitelistedlabels' on the top level layer 'pause' container,
// we have to use the 'group_left' functionality to join the labels we need back to the original container usage timeseries
// an example of this is the following
// avg by (part_of, container) ((rate(container_cpu_usage_seconds_total{container="nginx-random"})) + on (namespace,pod) group_left(part_of) ( 0 * container_cpu_usage_seconds_total{part_of!="",namespace="default"}))
func (ue *UsageEvaluator) buildUsageQuery(rateFilters []string,
	resourceType string, joinFilters []string, joinLabels []string) (string, error) {
	// 1. get the resource type metric label
	metricLabel, ok := schedv1alpha1.SupportedResourcesMetricLabel[resourceType]
	if !ok {
		return "", fmt.Errorf("unable to find the metric label")
	}

	var pquery string

	// 2. get the timewindow
	window, rok := schedv1alpha1.SupportedResourcesRateTimeWindow[resourceType]

	// 3. get the method
	method, mok := schedv1alpha1.SupportedResourcesRangeMethod[resourceType]

	// 4. additional custom filters
	additionalFilters, fok := schedv1alpha1.SupportedMetricLabelFilters[metricLabel]
	if fok {
		rateFilters = append(rateFilters, additionalFilters...)
	}

	pquery = metricLabel

	if len(rateFilters) > 0 {
		pquery = fmt.Sprintf("%s{%s}", pquery, strings.Join(rateFilters, ","))
	}

	if rok && mok {
		// add window when there is a rate method, so only add them if both okay
		pquery = fmt.Sprintf("%s(%s[%s])", method, pquery, window)
	}

	if len(joinFilters) > 0 && len(joinLabels) > 0 {
		// avg by (part_of, container) ((rate(container_cpu_usage_seconds_total{container="nginx-random"})) + on (namespace,pod) group_left(part_of) ( 0 * container_cpu_usage_seconds_total{part_of!="",namespace="default"}))
		pquery = fmt.Sprintf("avg by (%s,container) (%s + on (namespace,pod) group_left(%s) (0 * %s{%s}))",
			strings.Join(joinLabels, ","), pquery, strings.Join(joinLabels, ","), metricLabel, strings.Join(joinFilters, ","))
	}

	return pquery, nil
}

func (ue *UsageEvaluator) buildHistogram(values model.Value) (*dateTimeEstimator, error) {
	// TODO: Evaluate whether we should cache the estimator
	// Alternative is to create a LRU Histogram
	h, err := NewDateTimeEstimator()
	if err != nil {
		log.Error(err, "unable to create datetime histogram")
		return nil, err
	}

	// TODO: Evaluate overhead of the following approach
	// pulled all the series, then calculate each container runtime per series
	// if any one container run more than 24 hrs, then just add sample by hour
	// else, for each container series, shift the hour to the start of the day
	// at the end, mark the container as no long running
	// O(N) operation

	now := time.Now()
	maxWeek, isLongRunning, err := FindMaxWeekAndCheckIsLongRunning(values, now)
	if err != nil {
		log.Error(err, "error checking containers duration", "values", values)
		return h, err
	}

	// Step 2. O(N) for adding the samples
	if isLongRunning {
		err = AddWeightedSampleByWeek(h, maxWeek, values, now)
	} else {
		// we know the containers aren't long running.
		// shift each usage values to the start of the day,
		// the scheduler can then use the values for estimation however it see fits.
		// For example: if each container only runs for < 1hr,
		// The histogram will only contain values at Hour[0]
		// so the scheduler can estimate forecast by taking the non-zero values,
		// and add the values to the current time t.
		err = AddShiftedWeightedSampleByWeek(h, maxWeek, values, now)
	}

	return h, err
}

func (ue *UsageEvaluator) estimateHourUsage(ctx context.Context, ut *schedv1alpha1.UsageTemplate, h *dateTimeEstimator, resourceType string, scaleFactor float64) error {
	// default to 95 percentile to be conservative
	percentile := 0.95
	if ut.Spec.QualityOfServiceClass != string(corev1.PodQOSGuaranteed) {
		percentile = 0.5
	}

	resourceTypeUnit, ok := schedv1alpha1.SupportedResourceMetricUnit[resourceType]
	if !ok {
		// shouldn't have reached here
		return fmt.Errorf("resource metric unit is not supported")
	}

	samples := []schedv1alpha1.Sample{}
	for i := 0; i < len(h.Histograms); i++ {
		if h.Histograms[i].IsEmpty() {
			continue
		}
		// TODO: at the moment, our value is mostly using the cadvisor
		// so core seconds translating to millicore need to multiply by a scalefactor
		scaledValue := h.Histograms[i].Percentile(percentile) * scaleFactor
		sample := schedv1alpha1.Sample{
			Hour:       int32(h.Histograms[i].Hour),
			Value:      strconv.FormatFloat(scaledValue, 'f', -1, 64),
			Percentile: strconv.FormatFloat(percentile, 'f', -1, 64),
			Unit:       resourceTypeUnit,
			IsWeekday:  h.Histograms[i].IsWeekday,
		}
		samples = append(samples, sample)
	}

	// when patching we first create a new copy
	status := ut.Status.DeepCopy()

	// reset resource usages
	status.HistoricalUsage = &schedv1alpha1.ResourceUsages{
		Items: []schedv1alpha1.ResourceUsage{},
	}

	status.HistoricalUsage.Items = append(status.HistoricalUsage.Items, schedv1alpha1.ResourceUsage{
		Resource: resourceType,
		Usages:   samples,
	})

	status.IsLongRunning = h.IsLongRunning()

	return utils.UpdateStatus(ctx, ue.client, log, ut, status)
}

func (ue *UsageEvaluator) evaluateOne(ctx context.Context) {
	obj, err := ue.evaluationQ.Pop()
	if err != nil {
		log.Error(err, "unable to evaluate next usage template")
		return
	}

	qUT, ok := obj.(*tu.QueuedUsageTemplate)
	if !ok {
		log.Error(fmt.Errorf("expected queue usage template"), "error convert to queue usage template", "Obj", obj)
		return
	}

	if !qUT.UsageTemplate.Spec.Enabled {
		log.V(3).Info("Not necessary to evaluate the usage template", "enabled", qUT.Spec.Enabled, "usageTemplate", GetNamespacedName(qUT.UsageTemplate))
	}

	select {
	case <-qUT.Context.Done():
		{
			log.V(3).Info("context done, not evaluating", "usageTemplate", GetNamespacedName(qUT.UsageTemplate))
			return
		}
	default:
	}

	evaluateCtx, cancel := context.WithCancel(qUT.Context)
	defer cancel()

	if qUT.NextEvaluationTime.Before(ue.clock.Now()) {
		log.V(3).Info("attempting to evaluate usage template", "usageTemplate", GetNamespacedName(qUT.UsageTemplate), "EvaluatedCounts", qUT.Counts)

		ue.evaluateResources(evaluateCtx, log, qUT.UsageTemplate)
		qUT.Counts++

		intervalHour := qUT.UsageTemplate.Spec.EvaluatePeriodHours
		if intervalHour == nil {
			*intervalHour = schedv1alpha1.DefaultEvaluationPeriodHours
		}
		now := ue.clock.Now()
		qUT.NextEvaluationTime = now.Add(time.Duration(*intervalHour) * time.Hour)
		qUT.LastEvaluated = now
	} else {
		log.V(3).Info("Too early for", "usageTemplate", GetNamespacedName(qUT.UsageTemplate), "Next Evaluation Period", qUT.NextEvaluationTime.String())
	}

	ue.addToEvaluationQueue(qUT)
	log.V(3).Info("added back to evaluation q", "usageTemplate", GetNamespacedName(qUT.UsageTemplate))
}

func (ue *UsageEvaluator) Run(ctx context.Context) {
	log.Info("starting evaluation queue...")
	go wait.UntilWithContext(ctx, ue.evaluateOne, 15*time.Second)
	<-ctx.Done()
	ue.Close()
}

func (ue *UsageEvaluator) Close() {
	ue.mu.Lock()
	defer ue.mu.Unlock()
	ue.evaluationQ.Close()
}

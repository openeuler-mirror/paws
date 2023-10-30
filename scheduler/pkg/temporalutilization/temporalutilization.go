package temporalutilization

import (
	"context"
	"fmt"
	"math"

	pluginConfig "gitee.com/openeuler/paws/scheduler/apis/config"
	schedv1alpha1 "gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	pawsclientset "gitee.com/openeuler/paws/scheduler/pkg/generated/clientset/versioned"
	pawsformers "gitee.com/openeuler/paws/scheduler/pkg/generated/informers/externalversions"
	oc "gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/overcommit"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/apis/config"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/feature"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/noderesources"
)

const (
	Name          = "TemporalUtilization"
	SIRAnnotation = "sirlab"

	TRUEVALUE         = "true"
	ResourceSeparator = ":"
	CPUResource       = v1.ResourceCPU

	// DefaultHotSpotThreshold sets the hotspot threshold
	// Use the whole machine as default
	// TODO: Per machine type hotspot
	DefaultHotSpotThreshold = 100

	NumHoursInADay = 24

	// preFilterStateKey is the key in CycleState to NodeResourcesFit pre-computed data.
	// Using the name of the plugin will likely help us avoid collisions with other plugins.
	preFilterStateKey = "PreFilter" + Name
)

var DefaultResourceUnitMap = map[string]string{
	CPUResource.String(): "millicore",
}

// We follow NodeResourcesFit to compute pod resource request at prefilter
// preFilterState computed at PreFilter and used at Filter.
type preFilterState struct {
	framework.Resource
}

// Clone the prefilter state.
func (s *preFilterState) Clone() framework.StateData {
	return s
}

type TemporalUtilization struct {
	FitPlugin *noderesources.Fit

	HotSpotThreshold       int32
	HardThreshold          bool
	EnableOvercommit       bool
	FilterByTemporalUsages bool
	utMgr                  *UsageTemplateManager
}

var _ framework.PreFilterPlugin = &TemporalUtilization{}
var _ framework.FilterPlugin = &TemporalUtilization{}
var _ framework.ScorePlugin = &TemporalUtilization{}
var _ framework.ReservePlugin = &TemporalUtilization{}

func NewFitPlugin(handle framework.Handle) (*noderesources.Fit, error) {
	fArgs := config.NodeResourcesFitArgs{
		IgnoredResources:      []string{},
		IgnoredResourceGroups: []string{},
		ScoringStrategy: &config.ScoringStrategy{
			Type: config.MostAllocated,
		},
	}

	f, err := noderesources.NewFit(&fArgs, handle, feature.Features{})
	if err != nil {
		return nil, err
	}

	return f.(*noderesources.Fit), nil
}

func New(obj runtime.Object, handle framework.Handle) (framework.Plugin, error) {
	args, err := getArgs(obj)
	if err != nil {
		return nil, err
	}

	ctx := context.TODO()

	// setup informers for the usage template CRDs
	pawsClient := pawsclientset.NewForConfigOrDie(handle.KubeConfig())
	pawsInformerFactory := pawsformers.NewSharedInformerFactory(pawsClient, 0)
	utInformer := pawsInformerFactory.Scheduling().V1alpha1().UsageTemplates()
	podInformer := handle.SharedInformerFactory().Core().V1().Pods()

	handler := NewUsageTemplateManager(pawsClient, handle.SnapshotSharedLister(), utInformer, podInformer)

	pawsInformerFactory.Start(ctx.Done())

	hotspotThreshold := int32(DefaultHotSpotThreshold)
	// HotSpotThreshold must be greater than 0 and less than or equal to 100
	if args.HotSpotThreshold > 0 && args.HotSpotThreshold <= 100 {
		hotspotThreshold = args.HotSpotThreshold
	} else {
		err = fmt.Errorf("must be greater than one and less than or equal to a hundred")
		klog.ErrorS(err, "Using default hotspot threshold as 100, Expected between one and a hundred, got", "threshold", args.HotSpotThreshold)
	}

	enableOvercommit := args.EnableOvercommit

	f, err := NewFitPlugin(handle)
	if err != nil {
		klog.ErrorS(err, "unable to create fit plugin, defaulting to disable overcommit")
		enableOvercommit = false
	}

	pl := &TemporalUtilization{
		HotSpotThreshold:       hotspotThreshold,
		HardThreshold:          args.HardThreshold,
		EnableOvercommit:       enableOvercommit,
		FilterByTemporalUsages: args.FilterByTemporalUsages,
		utMgr:                  handler,
		FitPlugin:              f,
	}

	if !cache.WaitForCacheSync(ctx.Done(), utInformer.Informer().HasSynced) {
		err := fmt.Errorf("WaitForCacheSync failed")
		klog.ErrorS(err, "cannot sync caches")
		return nil, err
	}

	klog.V(3).InfoS("Tempora Utilization Plugin Intialized")

	return pl, nil
}

func (pl *TemporalUtilization) Name() string {
	return Name
}

func getArgs(obj runtime.Object) (*pluginConfig.TemporalUtilizationArgs, error) {
	args, ok := obj.(*pluginConfig.TemporalUtilizationArgs)
	if !ok {
		return nil, fmt.Errorf("want args to be of type TemporalUtilizationArgs, got %T", obj)
	}

	return args, nil
}

func (pl *TemporalUtilization) SupportedTargetResources() []string {
	results := []string{}
	for k, _ := range DefaultResourceUnitMap {
		results = append(results, k)
	}
	return results
}

// PreFilterExtensions returns prefilter extensions, pod add and remove.
func (pl *TemporalUtilization) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (pl *TemporalUtilization) preFilter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	cycleState.Write(preFilterStateKey, computePodResourceRequest(pod))
	return nil, nil
}

// PreFilter invoked at the prefilter extension point.
func (pl *TemporalUtilization) PreFilter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	if !pl.EnableOvercommit {
		return nil, nil
	}

	if pl.FilterByTemporalUsages {
		return pl.preFilter(ctx, cycleState, pod)
	}

	return pl.FitPlugin.PreFilter(ctx, cycleState, pod)
}

func (pl *TemporalUtilization) prepareNodeInfoForFilter(nodeInfo *framework.NodeInfo) (*framework.NodeInfo, error) {

	cloneNode := nodeInfo.Clone()
	if !pl.EnableOvercommit {
		return cloneNode, nil
	}

	resourceList, err := oc.CalculateOvercommitResources(nodeInfo, schedv1alpha1.SupportedOvercommitResourceAnnotation)
	if err != nil {
		return nil, err
	}

	cloneNode.Allocatable.Add(resourceList)

	klog.V(3).InfoS("Overcommitable", "Node", nodeInfo.Node().Name, "Allocatable", cloneNode.Allocatable)
	return cloneNode, nil
}

func (pl *TemporalUtilization) filterWithFit(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	cloneNode, err := pl.prepareNodeInfoForFilter(nodeInfo)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Errorf("node %v: %v ", nodeInfo.Node().Name, err).Error())
	}

	return pl.FitPlugin.Filter(ctx, cycleState, pod, cloneNode)
}

func (pl *TemporalUtilization) ignoredResources() sets.String {
	return sets.NewString(pl.SupportedTargetResources()...)
}

func (pl *TemporalUtilization) filterWithTemporalUsages(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	podResourcesRequested, err := getPreFilterState(cycleState)
	if err != nil {
		return framework.AsStatus(err)
	}

	cloneNode, err := pl.prepareNodeInfoForFilter(nodeInfo)
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Errorf("node %v: %v ", nodeInfo.Node().Name, err).Error())
	}

	// second, check whether other resources are within the capacity
	insufficientResources := fitsRequest(podResourcesRequested, cloneNode, pl.ignoredResources(), nil, nil)

	status := checkInsufficientResources(insufficientResources)

	if status != nil {
		return status
	}

	podTemporalUsages, err := getUsageTemplatesByPod(pl.utMgr, pod.Namespace, pod.Name, pl.SupportedTargetResources())
	if err != nil {
		return framework.NewStatus(framework.Error, fmt.Sprintf("unable to obtain usage template for pod: %s/%s", pod.Namespace, pod.Name))
	}

	// third, calculate forecasts on supported resources
	forecasts, msg, err := obtainForecasts(pl.utMgr, cloneNode, cloneNode.Node().Name, pod, pl.SupportedTargetResources())
	if err != nil {
		return framework.NewStatus(framework.Error, msg)
	}

	// fourth, filter using the forecasts over time
	insufficientResources = fitsRequestWithTemporal(podTemporalUsages, forecasts, cloneNode)

	return checkInsufficientResources(insufficientResources)
}

func (pl *TemporalUtilization) Filter(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	if pl.FilterByTemporalUsages {
		return pl.filterWithTemporalUsages(ctx, cycleState, pod, nodeInfo)
	}
	return pl.filterWithFit(ctx, cycleState, pod, nodeInfo)
}

func (pl *TemporalUtilization) Score(ctx context.Context, cycleState *framework.CycleState, pod *v1.Pod, nodeName string) (int64, *framework.Status) {
	// NOTE the assumption below:
	// 	  i.e. whoever puts the forecast there should beaware that which percentiles we are looking for
	// TODO: add more utils template if needed, only use cpu atm
	// just make sure we are looking at the right node
	nodeInfo, err := pl.utMgr.snapshotSharedLister.NodeInfos().Get(nodeName)
	if err != nil {
		return framework.MinNodeScore, framework.NewStatus(framework.Error,
			fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}

	forecasts, msg, err := obtainForecasts(pl.utMgr, nodeInfo, nodeName, pod, pl.SupportedTargetResources())
	if err != nil {
		return framework.MinNodeScore, framework.NewStatus(framework.Error, msg)
	}

	finalScore, status := scorer(nodeInfo, forecasts, pl.HotSpotThreshold, pl.HardThreshold)

	klog.V(6).InfoS("Temporal Score", "Score", finalScore, "Pod", klog.KObj(pod), "Node", klog.KObj(nodeInfo.Node()))

	return finalScore, status
}

func (pl *TemporalUtilization) ScoreExtensions() framework.ScoreExtensions {
	return pl
}

func (pl *TemporalUtilization) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	// Find highest and lowest scores.
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transform the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {

		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}

		klog.V(6).InfoS("Temporal Normalize Score", "Normalize Score", scores[i].Score, "Pod", klog.KObj(pod), "Node", scores[i].Name)
	}

	return nil
}

func (pl *TemporalUtilization) Reserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	pl.utMgr.addToCacheIfNotExists(pod, nodeName)
	// cannot fail
	return framework.NewStatus(framework.Success, "")
}

func (pl *TemporalUtilization) Unreserve(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) {
	pl.utMgr.deleteFromCacheIfExists(pod, nodeName)
}

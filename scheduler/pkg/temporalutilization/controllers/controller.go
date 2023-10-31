package controllers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"

	schedv1alpha1 "gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/evaluation"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/events"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/utils"
	v1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// only reconcile the above if the object podTemplateSpec has the annotation we are looking for
type UsageTemplateReconciler struct {
	Log      logr.Logger
	Recorder record.EventRecorder
	client.Client
	Scheme  *runtime.Scheme
	Workers int

	UsageEvaluator            *evaluation.UsageEvaluator
	usageTemplatesGenerations *sync.Map
}

// SetupWithManager initializes the UsageTemplateReconciler instance and starts a new controller managed by the passed Manager instance.
func (r *UsageTemplateReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options, globalHTTPTimeout time.Duration, evaluationResolution time.Duration, prometheusAddress string, ctx context.Context) error {
	var err error

	r.usageTemplatesGenerations = &sync.Map{}

	r.UsageEvaluator, err = evaluation.NewUsageEvaluator(mgr.GetClient(), mgr.GetScheme(), evaluationResolution, globalHTTPTimeout, r.Recorder, prometheusAddress)
	if err != nil {
		r.Log.Error(err, "unable to create usage evaluator")
		return err
	}

	go r.UsageEvaluator.Run(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		// Ignore updates to UsageTemplate Status (in this case metadata.Generation does not change)
		// so reconcile loop is not started on Status updates
		For(&schedv1alpha1.UsageTemplate{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=usagetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=usagetemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=usagetemplates/finalizers,verbs=update

func (r *UsageTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("reconciling usage template ...")
	ut := &schedv1alpha1.UsageTemplate{}
	if err := r.Get(ctx, req.NamespacedName, ut); err != nil {
		if apierrs.IsNotFound(err) {
			log.V(5).Info("Usage template does not exists")
			return ctrl.Result{}, nil
		}
		log.V(3).Error(err, "unable to retrieve usage template", "namespacedname", req.NamespacedName)
		return ctrl.Result{}, err
	}

	if ut.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.finalizeUsageTemplate(ctx, ut, req.NamespacedName.String())
	}

	r.updateCRDMetrics(ut, req.NamespacedName.String())

	// put finalizer on the CR to avoid GC
	if err := r.ensureFinalizer(ctx, ut); err != nil {
		return ctrl.Result{}, err
	}

	// update status conditions
	if !ut.Status.Conditions.AreReady() {
		err := utils.UpdateReadyConditions(ctx, r.Client, r.Log, ut, metav1.ConditionUnknown, "Initialized", "InitializedCondition")
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	// Reconcile UT, start evaluation loop
	msg, err := r.reconcileUsageTemplate(ctx, ut)

	r.Log.V(2).Info(msg, "Name", ut.Name, "Namespace", ut.Namespace)

	if err != nil {
		r.Log.Error(err, msg)
		utils.SetStatusConditions(ctx, r.Client, r.Log, ut, metav1.ConditionFalse, "UsageTemplateCheckFailed", msg, utils.TransformConditions)
		r.Recorder.Event(ut, v1.EventTypeWarning, events.CheckFailed, msg)
	} else if !ut.Spec.Enabled {
		utils.SetStatusConditions(ctx, r.Client, r.Log, ut, metav1.ConditionTrue, schedv1alpha1.DisabledSuccessReason, msg, utils.TransformConditions)
		r.Recorder.Event(ut, v1.EventTypeNormal, msg, "UsageTemplate is disabled")
	} else {
		utils.SetStatusConditions(ctx, r.Client, r.Log, ut, metav1.ConditionTrue, schedv1alpha1.ReadyForEvaluationSuccessReason, msg, utils.TransformConditions)
	}

	return ctrl.Result{}, nil
}

func (r *UsageTemplateReconciler) stopEvaluationLoop(ctx context.Context, ut *schedv1alpha1.UsageTemplate) error {
	key, err := cache.MetaNamespaceKeyFunc(ut)

	if err != nil {
		r.Log.Error(err, "Error getting key for usageTemplate")
		return err
	}

	if err := r.UsageEvaluator.DeleteUsageTemplateEvaluation(ctx, ut); err != nil {
		return err
	}

	// delete ut's current Generation
	r.usageTemplatesGenerations.Delete(key)
	return nil
}

func (r *UsageTemplateReconciler) reconcileUsageTemplate(ctx context.Context, ut *schedv1alpha1.UsageTemplate) (string, error) {
	if !ut.Spec.Enabled {
		r.UsageEvaluator.DeleteUsageTemplateEvaluation(ctx, ut)
		return schedv1alpha1.DisabledSuccessReason, nil
	}

	msg, err := r.validateUsageResourceTargets(ut)
	if err != nil {
		return msg, err
	}

	msg, err = r.validateEvaluationPeriods(ut)
	if err != nil {
		return msg, err
	}

	// Check object generation
	specChanged, err := r.usageTemplateGenerationChanged(ut)
	if err != nil {
		return "Failed to check whether UsageTemplate's Generation was changed", err
	}

	// Start Evaluation loop if all is well, e.g. the usagetemplate is changed or spec changed
	if specChanged {
		r.Recorder.Event(ut, v1.EventTypeNormal, events.ReadyForEvaluation, "UsageTemplate is ready for evaluation")

		if err := r.handleUsageTemplate(ctx, ut); err != nil {
			return "Failed to start a new evaluation loop", err
		}
		r.Log.Info("Started evaluation loop according to spec")
	}

	return schedv1alpha1.ReadyForEvaluationSuccessReason, nil
}

func (r *UsageTemplateReconciler) validateUsageResourceTargets(ut *schedv1alpha1.UsageTemplate) (string, error) {
	if len(ut.Spec.Resources) == 0 {
		return "No resources are specified", fmt.Errorf("expect at least one resource specified")
	}

	for _, resourceType := range ut.Spec.Resources {
		if _, ok := schedv1alpha1.SupportedResourcesMetricLabel[resourceType]; !ok {
			return "Resource not supported", fmt.Errorf("resource %s not supported yet. currently only supports %v", resourceType, schedv1alpha1.GetSupportedResources())
		}
	}

	return "", nil
}

func (r *UsageTemplateReconciler) validateEvaluationPeriods(ut *schedv1alpha1.UsageTemplate) (string, error) {
	if ut.Spec.EvaluatePeriodHours != nil && *ut.Spec.EvaluatePeriodHours < 1 {
		return "EvaluatePeriodHours out of range", fmt.Errorf("expect evaluate period hours to be greater than 0")
	}

	if ut.Spec.EvaluationWindowDays != nil && (*ut.Spec.EvaluationWindowDays < 1 || *ut.Spec.EvaluationWindowDays > 14) {
		return "EvaluatePeriodDays out of range", fmt.Errorf("expect evaluate period hours to be between [1,14]")
	}

	return "", nil
}

func (r *UsageTemplateReconciler) usageTemplateGenerationChanged(ut *schedv1alpha1.UsageTemplate) (bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(ut)
	if err != nil {
		r.Log.Error(err, "error getting namespace key for usage template")
		return true, err
	}

	value, loaded := r.usageTemplatesGenerations.Load(key)
	if loaded {
		generation := value.(int64)
		if generation == ut.Generation {
			return false, nil
		}
	}

	return true, nil
}

func (r *UsageTemplateReconciler) handleUsageTemplate(ctx context.Context, ut *schedv1alpha1.UsageTemplate) error {
	key, err := cache.MetaNamespaceKeyFunc(ut)
	if err != nil {
		r.Log.Error(err, "Error getting key for usage template")
		return err
	}

	if err = r.UsageEvaluator.HandleUsageTemplate(ctx, ut); err != nil {
		return err
	}

	if ut.Spec.Enabled {
		// store current generation to avoid starting a new evaluation
		r.usageTemplatesGenerations.Store(key, ut.Generation)
	}
	return nil
}

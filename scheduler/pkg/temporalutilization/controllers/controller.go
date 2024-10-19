package controllers

// Copyright (c) Huawei Technologies Co., Ltd. 2023-2024. All rights reserved.
// PAWS licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Wei Wei; Gingfung Yeung
// Date: 2024-10-18

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

// UsageTemplateReconciler reconciles a UsageTemplate object
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
		r.Log.Error(err, "Unable to create UsageEvaluator")
		return err
	}

	go r.UsageEvaluator.Run(ctx)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&schedv1alpha1.UsageTemplate{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Complete(r)
}

// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=usagetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=usagetemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=scheduling.x-k8s.io,resources=usagetemplates/finalizers,verbs=update

func (r *UsageTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling UsageTemplate...")

	ut := &schedv1alpha1.UsageTemplate{}
	if err := r.Get(ctx, req.NamespacedName, ut); err != nil {
		if apierrs.IsNotFound(err) {
			log.V(5).Info("UsageTemplate not found")
			return ctrl.Result{}, nil
		}
		log.V(3).Error(err, "Failed to retrieve UsageTemplate", "namespacedname", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// Finalization logic
	if ut.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, r.finalizeUsageTemplate(ctx, ut, req.NamespacedName.String())
	}

	r.updateCRDMetrics(ut, req.NamespacedName.String())

	// Ensure finalizer is present
	if err := r.ensureFinalizer(ctx, ut); err != nil {
		return ctrl.Result{}, err
	}

	// Update status conditions
	if !ut.Status.Conditions.AreReady() {
		if err := utils.UpdateReadyConditions(ctx, r.Client, r.Log, ut, metav1.ConditionUnknown, "Initialized", "InitializedCondition"); err != nil {
			return ctrl.Result{}, err
		}
	}

	msg, err := r.reconcileUsageTemplate(ctx, ut)
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
		r.Log.Error(err, "Error getting key for UsageTemplate")
		return err
	}

	if err := r.UsageEvaluator.DeleteUsageTemplateEvaluation(ctx, ut); err != nil {
		return err
	}

	// Delete UsageTemplate's current Generation
	r.usageTemplatesGenerations.Delete(key)
	return nil
}

func (r *UsageTemplateReconciler) reconcileUsageTemplate(ctx context.Context, ut *schedv1alpha1.UsageTemplate) (string, error) {
	if !ut.Spec.Enabled {
		r.UsageEvaluator.DeleteUsageTemplateEvaluation(ctx, ut)
		return schedv1alpha1.DisabledSuccessReason, nil
	}

	if msg, err := r.validateUsageResourceTargets(ut); err != nil {
		return msg, err
	}

	if msg, err := r.validateEvaluationPeriods(ut); err != nil {
		return msg, err
	}

	// Check object generation
	specChanged, err := r.usageTemplateGenerationChanged(ut)
	if err != nil {
		return "Failed to check whether UsageTemplate's Generation was changed", err
	}

	// Start evaluation loop if necessary
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
		return "No resources specified", fmt.Errorf("expect at least one resource specified")
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
		r.Log.Error(err, "Error getting namespace key for UsageTemplate")
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
		r.Log.Error(err, "Error getting key for UsageTemplate")
		return err
	}

	if err = r.UsageEvaluator.HandleUsageTemplate(ctx, ut); err != nil {
		return err
	}

	if ut.Spec.Enabled {
		// Store current generation to avoid starting a new evaluation
		r.usageTemplatesGenerations.Store(key, ut.Generation)
	}
	return nil
}


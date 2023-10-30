package controllers

import (
	"context"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/events"
	kedautil "github.com/kedacore/keda/v2/controllers/keda/util"
	corev1 "k8s.io/api/core/v1"
)

const (
	usageTemplateObjectFinalizer = "finalizer.sirlab.com"
)

func (r *UsageTemplateReconciler) finalizeUsageTemplate(ctx context.Context, ut *v1alpha1.UsageTemplate, namespacedName string) error {
	if kedautil.Contains(ut.GetFinalizers(), usageTemplateObjectFinalizer) {
		// run finalization logic and retry if unsuccessful
		if err := r.stopEvaluationLoop(ctx, ut); err != nil {
			r.Log.Error(err, "unable stop evaluation loop", "namespacedName", namespacedName)
			return nil
		}

		ut.SetFinalizers(kedautil.Remove(ut.GetFinalizers(), usageTemplateObjectFinalizer))
		if err := r.Client.Update(ctx, ut); err != nil {
			r.Log.Error(err, "Failed to update usageTemplate after removing a finalizer", "finalizer", usageTemplateObjectFinalizer)
			return err
		}

		r.updateCRDMetricsOnDelete(namespacedName)
	}

	r.Log.Info("successfully finalized UsageTemplate")
	r.Recorder.Event(ut, corev1.EventTypeNormal, events.Deleted, "object was deleted")
	return nil
}

func (r *UsageTemplateReconciler) ensureFinalizer(ctx context.Context, ut *v1alpha1.UsageTemplate) error {
	if !kedautil.Contains(ut.GetFinalizers(), usageTemplateObjectFinalizer) {
		r.Log.V(2).Info("Adding Finalizer to the UsageTemplate Object")
		ut.SetFinalizers(append(ut.GetFinalizers(), usageTemplateObjectFinalizer))

		err := r.Client.Update(ctx, ut)
		if err != nil {
			r.Log.Error(err, "Failed to update UsageTemplate with a finalizer", "finalizer", usageTemplateObjectFinalizer)
			return err
		}
	}
	return nil
}

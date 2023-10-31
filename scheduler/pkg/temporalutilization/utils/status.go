package utils

import (
	"context"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func PatchStatus(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, patch runtimeclient.Patch) error {
	err := client.Status().Patch(ctx, ut, patch)
	if err != nil {
		logger.Error(err, "unable to patch objects")
	}
	return err
}

func PatchTarget(ut *v1alpha1.UsageTemplate) runtimeclient.Patch {
	return runtimeclient.MergeFrom(ut.DeepCopy())
}

func SetStatusConditions(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status metav1.ConditionStatus, reason, message string, transform func(conditions *v1alpha1.Conditions, status metav1.ConditionStatus, reason string, message string)) error {
	patch := PatchTarget(ut)
	if len(ut.Status.Conditions) == 0 {
		ut.Status = v1alpha1.UsageTemplateStatus{
			Conditions: *v1alpha1.GetReadyCondtions(),
		}
	}
	transform(&ut.Status.Conditions, status, reason, message)
	logger.V(8).Info("Set status condition", "Status", ut.Status)
	return PatchStatus(ctx, client, logger, ut, patch)
}

// UpdateStatus patches the given UsageTemplate with the updated status passed to it or returns an error.
func UpdateStatus(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus) error {
	transform := func(ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus) {
		ut.Status = *status
	}
	return TransformAndPatch(ctx, client, logger, ut, status, transform)
}

// TransformAndPatch patches the given object with the targeted passed to it through a transformer function or returns an error.
func TransformAndPatch(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus, transform func(ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus)) error {
	patch := PatchTarget(ut)
	transform(ut, status)
	return PatchStatus(ctx, client, logger, ut, patch)
}

func TransformConditions(conditions *v1alpha1.Conditions, status metav1.ConditionStatus, reason string, message string) {
	conditions.SetReadyCondition(status, reason, message)
}

func UpdateReadyConditions(ctx context.Context, client runtimeclient.StatusClient, log logr.Logger, ut *v1alpha1.UsageTemplate, status metav1.ConditionStatus, reason, message string) error {
	err := SetStatusConditions(ctx, client, log, ut, status, reason, message, TransformConditions)
	if err != nil {
		log.Error(err, "Unable to update Status", "UsageTemplate", ut)
	}
	return err
}

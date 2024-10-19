package utils

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
// Date: 2024-10-19

import (
	"context"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// PatchStatus patches the status of a UsageTemplate object
func PatchStatus(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, patch runtimeclient.Patch) error {
	if err := client.Status().Patch(ctx, ut, patch); err != nil {
		logger.Error(err, "Unable to patch UsageTemplate status")
		return err
	}
	return nil
}

// PatchTarget creates a deep copy of the UsageTemplate for patching
func PatchTarget(ut *v1alpha1.UsageTemplate) runtimeclient.Patch {
	return runtimeclient.MergeFrom(ut.DeepCopy())
}

// SetStatusConditions sets the status conditions of a UsageTemplate and patches it
func SetStatusConditions(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status metav1.ConditionStatus, reason, message string, transform func(*v1alpha1.Conditions, metav1.ConditionStatus, string, string)) error {
	patch := PatchTarget(ut)

	if len(ut.Status.Conditions) == 0 {
		ut.Status.Conditions = *v1alpha1.GetReadyConditions()
	}

	transform(&ut.Status.Conditions, status, reason, message)
	logger.V(8).Info("Set status condition", "Status", ut.Status)
	return PatchStatus(ctx, client, logger, ut, patch)
}

// UpdateStatus updates the entire UsageTemplate status by patching
func UpdateStatus(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus) error {
	return TransformAndPatch(ctx, client, logger, ut, status, func(ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus) {
		ut.Status = *status
	})
}

// TransformAndPatch applies the transformation and patches the UsageTemplate status
func TransformAndPatch(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status *v1alpha1.UsageTemplateStatus, transform func(*v1alpha1.UsageTemplate, *v1alpha1.UsageTemplateStatus)) error {
	patch := PatchTarget(ut)
	transform(ut, status)
	return PatchStatus(ctx, client, logger, ut, patch)
}

// TransformConditions updates the ready condition of the UsageTemplate
func TransformConditions(conditions *v1alpha1.Conditions, status metav1.ConditionStatus, reason, message string) {
	conditions.SetReadyCondition(status, reason, message)
}

// UpdateReadyConditions updates the ready condition in the UsageTemplate and patches it
func UpdateReadyConditions(ctx context.Context, client runtimeclient.StatusClient, logger logr.Logger, ut *v1alpha1.UsageTemplate, status metav1.ConditionStatus, reason, message string) error {
	return SetStatusConditions(ctx, client, logger, ut, status, reason, message, TransformConditions)
}



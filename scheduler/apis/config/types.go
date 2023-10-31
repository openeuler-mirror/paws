// Copyright (c) Huawei Technologies Co., Ltd. 2023. All rights reserved.
// paws licensed under the Mulan PSL v2.
// You can use this software according to the terms and conditions of the Mulan PSL v2.
// You may obtain a copy of Mulan PSL v2 at:
//     http://license.coscl.org.cn/MulanPSL2
// THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND, EITHER EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT, MERCHANTABILITY OR FIT FOR A PARTICULAR
// PURPOSE.
// See the Mulan PSL v2 for more details.
// Author: Wei Wei
// Create: 2023-09-18

package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemporalUtilizationArgs holds arguments used to configure TemporalUtilization plugin.
type TemporalUtilizationArgs struct {
	metav1.TypeMeta

	// HotSpot threshold for disallowing scores
	HotSpotThreshold int32

	// HardThreshold is a flag to indicate whether the threshold is hard or soft
	// If it is hard, we will not allow any pod to be scheduled on the node if the threshold is reached
	// If it is soft, we will allow pods to be scheduled on the node even if the threshold is reached, and reduce the score of the node
	HardThreshold bool

	// EnableOvercommit is a flag to indicate whether the plugin conducts overcommit at the filtering stage
	EnableOvercommit bool

	// FilterByTemporalUsages is a flag to indicate whether the plugin conducts filtering stage by temporal usages if present
	FilterByTemporalUsages bool
}

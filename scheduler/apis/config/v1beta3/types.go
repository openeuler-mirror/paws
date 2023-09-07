/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetricProviderType is a "string" type.
type MetricProviderType string

const (
	KubernetesMetricsServer MetricProviderType = "KubernetesMetricsServer"
	Prometheus              MetricProviderType = "Prometheus"
)

// Denote the spec of the metric provider
type MetricProviderSpec struct {
	// Types of the metric provider
	Type MetricProviderType `json:"type,omitempty"`
	// The address of the metric provider
	Address *string `json:"address,omitempty"`
	// The authentication token of the metric provider
	Token *string `json:"token,omitempty"`
	// Whether to enable the InsureSkipVerify options for https requests on Metric Providers.
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true
type TargetMetricSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// MetricLabel from the metric provider
	MetricLabel *string `json:"metricLabel,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemporalUtilizationArgs holds arguments used to configure TemporalUtilization plugin.
type TemporalUtilizationArgs struct {
	metav1.TypeMeta

	// BestEffortPercentile value to sample for the BE jobs
	BestEffortPercentile *int32 `json:"bestEffortPercentile,omitempty"`
	// LatencyCriticalPercentile value to sample for the LC jobs
	LatencyCriticalPercentile *int32 `json:"latencyCriticalPercentile,omitempty"`

	// Metric Provider to use when using load watcher as a library
	MetricProvider MetricProviderSpec `json:"metricProvider,omitempty"`
	// Address of load watcher service
	WatcherAddress *string `json:"watcherAddress,omitempty"`
	// TargetMetrics to query the metric provider for
	TargetMetrics []TargetMetricSpec `json:"targetMetrics,omitempty"`
	// HotSpot threshold for disallowing scores
	HotSpotThreshold *float64 `json:"hotSpotThreshold,omitempty"`
}


package config

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
	Type MetricProviderType
	// The address of the metric provider
	Address string
	// The authentication token of the metric provider
	Token string
	// Whether to enable the InsureSkipVerify options for https requests on Metric Providers.
	InsecureSkipVerify bool
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

type TargetMetricSpec struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// MetricLabel from the metric provider
	MetricLabel string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TemporalUtilizationArgs holds arguments used to configure TemporalUtilization plugin.
type TemporalUtilizationArgs struct {
	metav1.TypeMeta

	// BestEffortPercentile value to sample for the BE jobs
	BestEffortPercentile int32
	// LatencyCriticalPercentile value to sample for the LC jobs
	LatencyCriticalPercentile int32

	// Metric Provider to use when using load watcher as a library
	MetricProvider MetricProviderSpec
	// Address of load watcher service
	WatcherAddress string
	// TargetMetrics to query the metric provider for
	TargetMetrics []TargetMetricSpec
	// HotSpot threshold for disallowing scores
	HotSpotThreshold float64
}


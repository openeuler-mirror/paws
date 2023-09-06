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

// TemporalUtilizationArgs holds arguments used to configure TemporalUtilization plugin.
type TemporalUtilizationArgs struct {
	metav1.TypeMeta

	// BEPercentile value to sample for the BE jobs
	BEPercentile int32
	// LCPercentile value to sample for the LC jobs
	LCPercentile int32

	// Metric Provider to use when using load watcher as a library
	MetricProvider MetricProviderSpec
	// Address of load watcher service
	WatcherAddress string
	// TargetMetricLabels for a list of labels to fetch
	TargetMetricLabels []string
	// HotSpot threshold for disallowing scores
	HotSpotThreshold float64
}

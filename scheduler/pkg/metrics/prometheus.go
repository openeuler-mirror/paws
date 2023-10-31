package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var log = logf.Log.WithName("controller_prometheus")

const (
	// TODO: verify whether the controller should be in different namespace
	DefaultControllerNamespace = "paws"
	UsageTemplateType          = "usage_template"
)

var (
	metricLabels        = []string{"namespace", "metric", "usageTemplate"}
	usageTemplateActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: DefaultControllerNamespace,
			Subsystem: "usage_template",
			Name:      "active",
			Help:      "Number of Usage Template evaluation that is active",
		},
		metricLabels,
	)

	resourceTotalsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: DefaultControllerNamespace,
			Subsystem: "resource",
			Name:      "totals",
		},
		[]string{"type"},
	)

	crdTotalsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: DefaultControllerNamespace,
			Subsystem: "resource",
			Name:      "totals",
		},
		[]string{"type", "namespace"},
	)
)

// init executes all the metrics registration when the package is loaded
func init() {
	metrics.Registry.MustRegister(crdTotalsGaugeVec)

}

func DecrementCRDTotal(crdType, namespace string) {
	if namespace == "" {
		namespace = "default"
	}

	crdTotalsGaugeVec.WithLabelValues(crdType, namespace).Dec()
}

func DecrementResourceTotal(resourceType string) {
	if resourceType != "" {
		resourceTotalsGaugeVec.WithLabelValues(resourceType).Dec()
	}
}

func IncrementCRDTotal(crdType, namespace string) {
	if namespace == "" {
		namespace = "default"
	}
	crdTotalsGaugeVec.WithLabelValues(crdType, namespace).Inc()
}

func IncrementResourceTotal(resourceType string) {
	if resourceType != "" {
		resourceTotalsGaugeVec.WithLabelValues(resourceType).Inc()
	}
}

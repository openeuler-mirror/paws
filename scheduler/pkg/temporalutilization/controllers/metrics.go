package controllers

import (
	"sync"

	schedv1alpha1 "gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	prommetrics "gitee.com/openeuler/paws/scheduler/pkg/metrics"
)

var (
	promMetricsMap map[string]controllerMetricData
	promMetricsMu  *sync.Mutex
)

type controllerMetricData struct {
	namespace string
	resources []string
}

func init() {
	promMetricsMap = make(map[string]controllerMetricData)
	promMetricsMu = &sync.Mutex{}
}

func (r *UsageTemplateReconciler) updateCRDMetrics(ut *schedv1alpha1.UsageTemplate, namespacedName string) {
	promMetricsMu.Lock()
	defer promMetricsMu.Unlock()

	metricsData, ok := promMetricsMap[namespacedName]
	if ok {
		prommetrics.DecrementCRDTotal(prommetrics.UsageTemplateType, metricsData.namespace)
		for _, resourceType := range metricsData.resources {
			prommetrics.DecrementResourceTotal(resourceType)
		}
	}

	prommetrics.IncrementCRDTotal(prommetrics.UsageTemplateType, ut.Namespace)
	metricsData.namespace = ut.Namespace

	resourceTypes := make([]string, len(ut.Spec.Resources))
	for _, resourceType := range ut.Spec.Resources {
		prommetrics.IncrementResourceTotal(resourceType)
		resourceTypes = append(resourceTypes, resourceType)
	}

	metricsData.resources = resourceTypes
	promMetricsMap[namespacedName] = metricsData
}

func (r *UsageTemplateReconciler) updateCRDMetricsOnDelete(namespacedName string) {
	promMetricsMu.Lock()
	defer promMetricsMu.Unlock()

	if metricsData, ok := promMetricsMap[namespacedName]; ok {
		prommetrics.DecrementCRDTotal(prommetrics.UsageTemplateType, metricsData.namespace)
		for _, resourceType := range metricsData.resources {
			prommetrics.DecrementResourceTotal(resourceType)
		}
	}

	delete(promMetricsMap, namespacedName)
}

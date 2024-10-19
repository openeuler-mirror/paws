package metrics

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
// Description: This file is used for metrics collections


import (
	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var log = logf.Log.WithName("controller_prometheus")

const (
	DefaultControllerNamespace = "paws"
	DefaultNamespace           = "default" // 提取默认命名空间为常量
	UsageTemplateType          = "usage_template"
)

var (
	metricLabels = []string{"namespace", "metric", "usageTemplate"}

	usageTemplateActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: DefaultControllerNamespace,
			Subsystem: "usage_template",
			Name:      "active",
			Help:      "Number of Usage Template evaluations that are active",
		},
		metricLabels,
	)

	resourceTotalsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: DefaultControllerNamespace,
			Subsystem: "resource",
			Name:      "totals",
			Help:      "Total number of resources", // 加入 Help 信息
		},
		[]string{"type"},
	)

	crdTotalsGaugeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: DefaultControllerNamespace,
			Subsystem: "resource",
			Name:      "totals",
			Help:      "Total number of CRDs by type and namespace", // 加入 Help 信息
		},
		[]string{"type", "namespace"},
	)
)

// init executes all the metrics registration when the package is loaded
func init() {
	metrics.Registry.MustRegister(crdTotalsGaugeVec, resourceTotalsGaugeVec) // 注册所有 metrics
	log.Info("Prometheus metrics registered")
}

// adjustGauge adjusts the gauge value (increment/decrement) for a CRD or resource
func adjustGauge(gaugeVec *prometheus.GaugeVec, labels []string, increment bool) {
	if increment {
		gaugeVec.WithLabelValues(labels...).Inc()
	} else {
		gaugeVec.WithLabelValues(labels...).Dec()
	}
}

// DecrementCRDTotal decrements the total count of a CRD for the specified type and namespace
func DecrementCRDTotal(crdType, namespace string) {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	adjustGauge(crdTotalsGaugeVec, []string{crdType, namespace}, false)
	log.Info("Decremented CRD total", "type", crdType, "namespace", namespace)
}

// IncrementCRDTotal increments the total count of a CRD for the specified type and namespace
func IncrementCRDTotal(crdType, namespace string) {
	if namespace == "" {
		namespace = DefaultNamespace
	}
	adjustGauge(crdTotalsGaugeVec, []string{crdType, namespace}, true)
	log.Info("Incremented CRD total", "type", crdType, "namespace", namespace)
}

// DecrementResourceTotal decrements the total count of a resource for the specified type
func DecrementResourceTotal(resourceType string) {
	if resourceType != "" {
		adjustGauge(resourceTotalsGaugeVec, []string{resourceType}, false)
		log.Info("Decremented resource total", "type", resourceType)
	}
}

// IncrementResourceTotal increments the total count of a resource for the specified type
func IncrementResourceTotal(resourceType string) {
	if resourceType != "" {
		adjustGauge(resourceTotalsGaugeVec, []string{resourceType}, true)
		log.Info("Incremented resource total", "type", resourceType)
	}
}

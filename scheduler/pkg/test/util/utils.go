package util

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
// Description: This file is used for usage template creation

import (
	"sort"
	"strconv"
	"strings"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Make usages across periods according to the format
// [[start,end,usages]], where each period would be [start,end)
func MakeUsageAcrossPeriods(periodUsages [][]float32) map[int]float32 {
	results := make(map[int]float32)

	// Sort the usage periods by the start time
	sort.Slice(periodUsages, func(i, j int) bool {
		return periodUsages[i][0] < periodUsages[j][0]
	})

	for _, usagePeriod := range periodUsages {
		// Ensure that each period has exactly 3 values (start, end, usage)
		if len(usagePeriod) != 3 {
			continue
		}
		start, end := int(usagePeriod[0]), int(usagePeriod[1])
		usages := usagePeriod[2]

		for i := start; i < end; i++ {
			results[i] = usages
		}
	}
	return results
}

// Create a map of usage for all 24 hours of the day with the same usage value
func SameUsageADay(usage float32) map[int]float32 {
	// Pre-allocate map size for better performance
	results := make(map[int]float32, 24)
	for i := 0; i < 24; i++ {
		results[i] = usage
	}
	return results
}

// Convert resource usage data into the appropriate scheduling resource usage format
func MakeResourceUsages(resourceUsages map[string]map[int]float32, resources map[string]bool, isWeekday bool) []v1alpha1.ResourceUsage {
	schedResourceUsages := make([]v1alpha1.ResourceUsage, 0, len(resourceUsages))

	for res, usages := range resourceUsages {
		// Track the resources being used
		resources[res] = true

		// Build sample data for each resource
		samples := make([]v1alpha1.Sample, 0, len(usages))
		for hour, value := range usages {
			samples = append(samples, v1alpha1.Sample{
				Hour:      int32(hour),
				Value:     strconv.FormatFloat(float64(value), 'f', 2, 32),
				IsWeekday: isWeekday,
			})
		}

		// Add the resource usage to the list
		schedResourceUsages = append(schedResourceUsages, v1alpha1.ResourceUsage{
			Resource: res,
			Usages:   samples,
		})
	}

	return schedResourceUsages
}

// Create a usage template for resources with their weekday and weekend usages
func MakeUsageTemplate(name, namespace string, enabled bool, qosClass string, resourceWeekdayUsages, resourceWeekendUsages map[string]map[int]float32, isLongRunning bool) *v1alpha1.UsageTemplate {
	resources := make(map[string]bool)

	// Collect both weekday and weekend usages
	schedResourceUsages := append(MakeResourceUsages(resourceWeekdayUsages, resources, true),
		MakeResourceUsages(resourceWeekendUsages, resources, false)...)

	// Create a slice of resource names
	resourceStrings := make([]string, 0, len(resources))
	for res := range resources {
		resourceStrings = append(resourceStrings, res)
	}

	// Construct and return the usage template
	return &v1alpha1.UsageTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: v1alpha1.UsageTemplateSpec{
			Enabled:               enabled,
			Resources:             resourceStrings,
			QualityOfServiceClass: qosClass,
		},
		Status: v1alpha1.UsageTemplateStatus{
			IsLongRunning: isLongRunning,
			HistoricalUsage: &v1alpha1.ResourceUsages{
				Items: schedResourceUsages,
			},
		},
	}
}

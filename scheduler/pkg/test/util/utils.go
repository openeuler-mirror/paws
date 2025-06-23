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
    
    // 提前过滤无效数据并排序
    validPeriods := filterAndSortPeriods(periodUsages)
    
    for _, period := range validPeriods {
        start, end, usage := int(period[0]), int(period[1]), period[2]
        
        // 处理时间范围重叠的情况（取最大值）
        for hour := start; hour < end; hour++ {
            // 如果时间点已存在且当前值更大，则更新
            if existing, ok := results[hour]; !ok || usage > existing {
                results[hour] = usage
            }
        }
    }
    
    return results
}

// 过滤无效周期并按开始时间排序
func filterAndSortPeriods(periods [][]float32) [][]float32 {
    valid := make([][]float32, 0, len(periods))
    
    for _, p := range periods {
        // 验证周期数据有效性
        if len(p) != 3 {
            continue // 无效数据，跳过
        }
        
        start, end := int(p[0]), int(p[1])
        // 过滤掉开始时间大于等于结束时间的无效周期
        if start >= end {
            continue
        }
        
        valid = append(valid, p)
    }
    
    // 按开始时间排序
    sort.Slice(valid, func(i, j int) bool {
        return valid[i][0] < valid[j][0]
    })
    
    return valid
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
    if resourceUsages == nil {
        return nil
    }
    
    schedResourceUsages := make([]v1alpha1.ResourceUsage, 0, len(resourceUsages))
    
    for res, usages := range resourceUsages {
        // Track the resources being used
        resources[res] = true
        
        // Build sample data for each resource with error handling
        samples := buildUsageSamples(usages, isWeekday)
        
        // Add the resource usage to the list
        schedResourceUsages = append(schedResourceUsages, v1alpha1.ResourceUsage{
            Resource: res,
            Usages:   samples,
        })
    }
    
    return schedResourceUsages
}

func buildUsageSamples(hourlyValues map[int]float32, isWeekday bool) []v1alpha1.Sample {
    if hourlyValues == nil {
        return nil
    }
    
    samples := make([]v1alpha1.Sample, 0, len(hourlyValues))
    
    for hour, value := range hourlyValues {
        // 过滤无效小时值（保留有效数据）
        if hour < 0 || hour > 23 {
            continue // 或者记录日志：log.Printf("Invalid hour %d for resource usage", hour)
        }
        
        // 格式化浮点数，确保精度
        valueStr := strconv.FormatFloat(float64(value), 'f', 2, 32)
        
        samples = append(samples, v1alpha1.Sample{
            Hour:      int32(hour),
            Value:     valueStr,
            IsWeekday: isWeekday,
        })
    }
    
    return samples
}    


// Create a usage template for resources with their weekday and weekend usages
func MakeUsageTemplate(name, namespace string, enabled bool, qosClass string,
    resourceWeekdayUsages, resourceWeekendUsages map[string]map[int]float32,
    isLongRunning bool) (*v1alpha1.UsageTemplate, error) {

    // 收集所有资源名称
    resourceSet := make(map[string]struct{})
    collectResourceNames(resourceWeekdayUsages, resourceSet)
    collectResourceNames(resourceWeekendUsages, resourceSet)

    // 转换资源为切片
    resourceStrings := make([]string, 0, len(resourceSet))
    for res := range resourceSet {
        resourceStrings = append(resourceStrings, res)
    }

    // 处理工作日和周末
    weekdaySchedUsages, err := MakeResourceUsages(resourceWeekdayUsages, true)
    if err != nil {
        return nil, fmt.Errorf("failed to process weekday usage data: %w", err)
    }

    weekendSchedUsages, err := MakeResourceUsages(resourceWeekendUsages, false)
    if err != nil {
        return nil, fmt.Errorf("failed to process weekend usage data: %w", err)
    }

    // 合并资源使用数据
    schedResourceUsages := append(weekdaySchedUsages, weekendSchedUsages...)

    // 构建并返回 UsageTemplate 对象
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
    }, nil
}

func collectResourceNames(usageData map[string]map[int]float32, resourceSet map[string]struct{}) {
    for resourceName := range usageData {
        resourceSet[resourceName] = struct{}{}
    }
}

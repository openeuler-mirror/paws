package temporalutilization

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
	"math"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// calculateHourScore 计算单个小时的分数
func calculateHourScore(usagePercent float64, thresholdPercent float64, isHardConstraint bool) float64 {
	if usagePercent >= thresholdPercent {
		if isHardConstraint {
			return float64(framework.MinNodeScore)
		}
		// 软性约束时，使用惩罚计算
		return thresholdPercent * (float64(framework.MaxNodeScore) - usagePercent) /
			(float64(framework.MaxNodeScore) - thresholdPercent)
	}
	// 如果小于阈值，按比例计算得分
	return usagePercent/thresholdPercent*float64(framework.MaxNodeScore-thresholdPercent) + thresholdPercent
}

// scoreOverHours 计算给定时间段内的节点得分
func scoreOverHours(hoursValue map[int16]float32, nodeCapacityMilli int64, hotspotThresholdPercent int64, isHardConstraint bool) int64 {
	total := 0.0
	for _, millivalue := range hoursValue {
		usagePercent := float64(millivalue) / float64(nodeCapacityMilli) * 100.0
		hourScore := calculateHourScore(usagePercent, float64(hotspotThresholdPercent), isHardConstraint)
		total += math.Round(hourScore)
	}
	return int64(math.Round(total))
}

// getTrimaranScore 计算周内与周末的平均得分
func getTrimaranScore(nodeCapacityMilli int64, forecast *UsageTemplate, hotspotThresholdPercent int64, isHardConstraint bool) int64 {
	weekdayScore := scoreOverHours(forecast.weekDayHour, nodeCapacityMilli, hotspotThresholdPercent, isHardConstraint)
	weekendScore := scoreOverHours(forecast.weekendHour, nodeCapacityMilli, hotspotThresholdPercent, isHardConstraint)
	return int64(math.Round(float64(weekdayScore + weekendScore)))
}

// scorer 是打分函数，根据节点的资源使用情况和预测返回最终分数
func scorer(nodeInfo *framework.NodeInfo, forecasts map[string]*UsageTemplate, hotspotThreshold int32, isHardConstraint bool) (int64, *framework.Status) {
	nodeCPUMilli := nodeInfo.Node().Status.Capacity.Cpu().MilliValue()
	totalScore := int64(0)
	resourceCount := 0

	// 目前仅支持CPU
	for resource, template := range forecasts {
		if resource == v1.ResourceCPU.String() {
			perResourceScore := getTrimaranScore(nodeCPUMilli, template, int64(hotspotThreshold), isHardConstraint)
			totalScore += perResourceScore
			resourceCount++
		} else {
			klog.InfoS("Unsupported resource type", "Resource", resource)
		}
	}

	if resourceCount == 0 {
		klog.ErrorS(nil, "No supported resources found for scoring")
		return framework.MinNodeScore, framework.NewStatus(framework.Error, "No supported resources found")
	}

	// 返回所有资源的平均得分
	finalScore := int64(math.Round(float64(totalScore) / float64(resourceCount)))
	return finalScore, framework.NewStatus(framework.Success, "")
}

package temporalutilization

import (
	"math"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func scoreOverHours(hoursValue map[int16]float32, nodeCapacityMilli int64, hotspotThresholdPercent int64, isHardConstraint bool) int64 {
	total := 0.0
	hourScore := 0.0
	// NOTE: For now this is only for CPU
	for _, millivalue := range hoursValue {

		usagePercent := float64(millivalue) / float64(nodeCapacityMilli) * 100.0

		if usagePercent >= float64(hotspotThresholdPercent) {
			if isHardConstraint {
				return framework.MinNodeScore
			}
			// if its soft then we just use a penalised score
			hourScore = math.Round(
				float64(hotspotThresholdPercent) *
					(float64(framework.MaxNodeScore) - usagePercent) /
					(float64(framework.MaxNodeScore - int64(hotspotThresholdPercent))))
		} else {
			// less than the thresholdmilli
			hourScore = math.Round(usagePercent/float64(hotspotThresholdPercent)*
				float64(framework.MaxNodeScore-hotspotThresholdPercent) + float64(hotspotThresholdPercent))
		}
		total += hourScore
	}

	return int64(math.Round(total))
}

func getTrimaranScore(nodeCapacityMilli int64, forecast *UsageTemplate, hotspotThresholdPercent int64, isHardConstraint bool) int64 {
	weekdayScores := scoreOverHours(forecast.weekDayHour, nodeCapacityMilli, hotspotThresholdPercent, isHardConstraint)
	weekendScores := scoreOverHours(forecast.weekendHour, nodeCapacityMilli, hotspotThresholdPercent, isHardConstraint)

	return int64(math.Round((float64(weekdayScores) + float64(weekendScores))))
}

// TODO:
// Only CPU atm
// Some hours might have different thresholds
// Some hours might have higher penalise constraint
// Evaluate what is a good score, options are:
// i) Trimaran (current)
// ii) No Score once its over the threshold (Hard constraint)
func scorer(nodeInfo *framework.NodeInfo, forecasts map[string]*UsageTemplate, hotspotThreshold int32, isHardConstraint bool) (int64, *framework.Status) {
	nodeCPUMilli := nodeInfo.Node().Status.Capacity.Cpu().MilliValue()

	total := int64(0)
	totalResource := 0
	for resource, template := range forecasts {
		if resource == v1.ResourceCPU.String() {
			// NOTE: This part should be switchable in the future
			perResourceScore := getTrimaranScore(nodeCPUMilli, template, int64(hotspotThreshold), isHardConstraint)
			total += perResourceScore
			totalResource += 1
		} else {
			klog.InfoS("currently only supports CPU resource", "Resource", resource)
			continue
		}
	}

	finalScore := int64(math.Round(float64(total / int64(totalResource))))
	return finalScore, framework.NewStatus(framework.Success, "")
}

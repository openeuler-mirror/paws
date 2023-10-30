package util

import (
	"sort"
	"strconv"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Make usages across periods according to the format
// [[start,end,usages]], where each period would be [start,end)
func MakeUsageAcrossPeriods(periodUsages [][]float32) map[int]float32 {
	results := map[int]float32{}

	sort.Slice(periodUsages, func(i, j int) bool {
		return periodUsages[i][0] < periodUsages[j][0]
	})

	for _, usagePeriod := range periodUsages {
		if len(usagePeriod) != 3 {
			// Ignore emitting errors for now
			continue
		}
		start := int(usagePeriod[0])
		end := int(usagePeriod[1])
		usages := usagePeriod[2]

		for i := start; i < end; i++ {
			results[i] = usages
		}
	}
	return results
}

func SameUsageADay(usage float32) map[int]float32 {
	results := map[int]float32{}
	for i := 0; i < 24; i++ {
		results[i] = usage
	}
	return results
}

func MakeResourceUsages(resourceUsages map[string]map[int]float32, resources map[string]bool, isweekday bool) []v1alpha1.ResourceUsage {
	schedResourceUsages := []v1alpha1.ResourceUsage{}

	for res, usages := range resourceUsages {
		resources[res] = true

		samples := []v1alpha1.Sample{}
		for hour, value := range usages {
			samples = append(samples, v1alpha1.Sample{
				Hour:      int32(hour),
				Value:     strconv.FormatFloat(float64(value), 'f', 2, 32),
				IsWeekday: isweekday,
			})
		}

		schedResourceUsages = append(schedResourceUsages, v1alpha1.ResourceUsage{
			Resource: res,
			Usages:   samples,
		})
	}

	return schedResourceUsages
}

// resource map[string]map[int]float32 => "cpu": {1: 0.5, 2:0.6 ....}
// indicates each resource, usage in each period (hour)
func MakeUsageTemplate(name, namespace string, enabled bool, qosClass string, resourceWeekdayUsages, resourceWeekendUsages map[string]map[int]float32, isLongRunning bool) *v1alpha1.UsageTemplate {
	resources := make(map[string]bool)
	schedResourceUsages := []v1alpha1.ResourceUsage{}
	schedResourceUsages = append(schedResourceUsages, MakeResourceUsages(resourceWeekdayUsages, resources, true)...)
	schedResourceUsages = append(schedResourceUsages, MakeResourceUsages(resourceWeekendUsages, resources, false)...)

	resourceStrings := []string{}
	for res := range resources {
		resourceStrings = append(resourceStrings, res)
	}

	ut := &v1alpha1.UsageTemplate{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
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
	return ut
}

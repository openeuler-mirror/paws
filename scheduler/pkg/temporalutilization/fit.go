package temporalutilization

import (
	"fmt"
	"strings"

	"math"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/noderesources"
)

// From NodeResourcesFit
// computePodResourceRequest returns a framework.Resource that covers the largest
// width in each resource dimension. Because init-containers run sequentially, we collect
// the max in each dimension iteratively. In contrast, we sum the resource vectors for
// regular containers since they run simultaneously.
//
// # The resources defined for Overhead should be added to the calculated Resource request sum
//
// Example:
//
// Pod:
//
//	InitContainers
//	  IC1:
//	    CPU: 2
//	    Memory: 1G
//	  IC2:
//	    CPU: 2
//	    Memory: 3G
//	Containers
//	  C1:
//	    CPU: 2
//	    Memory: 1G
//	  C2:
//	    CPU: 1
//	    Memory: 1G
//
// Result: CPU: 3, Memory: 3G
func computePodResourceRequest(pod *v1.Pod) *preFilterState {
	result := &preFilterState{}
	for _, container := range pod.Spec.Containers {
		result.Add(container.Resources.Requests)
	}

	// take max_resource(sum_pod, any_init_container)
	for _, container := range pod.Spec.InitContainers {
		result.SetMaxResource(container.Resources.Requests)
	}

	// If Overhead is being utilized, add to the total requests for the pod
	if pod.Spec.Overhead != nil {
		result.Add(pod.Spec.Overhead)
	}
	return result
}

func getPreFilterState(cycleState *framework.CycleState) (*preFilterState, error) {
	c, err := cycleState.Read(preFilterStateKey)
	if err != nil {
		// preFilterState doesn't exist, likely PreFilter wasn't invoked.
		return nil, fmt.Errorf("error reading %q from cycleState: %w", preFilterStateKey, err)
	}

	s, ok := c.(*preFilterState)
	if !ok {
		return nil, fmt.Errorf("%+v  convert to NodeResourcesFit.preFilterState error", c)
	}
	return s, nil
}

func fitsRequest(podRequest *preFilterState, nodeInfo *framework.NodeInfo, ignoreResources, ignoredExtendedResources, ignoredResourceGroups sets.String) []noderesources.InsufficientResource {
	insufficientResources := make([]noderesources.InsufficientResource, 0, 4)

	allowedPodNumber := nodeInfo.Allocatable.AllowedPodNumber
	if len(nodeInfo.Pods)+1 > allowedPodNumber {
		insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
			ResourceName: v1.ResourcePods,
			Reason:       "Too many pods",
			Requested:    1,
			Used:         int64(len(nodeInfo.Pods)),
			Capacity:     int64(allowedPodNumber),
		})
	}

	if podRequest.MilliCPU == 0 &&
		podRequest.Memory == 0 &&
		podRequest.EphemeralStorage == 0 &&
		len(podRequest.ScalarResources) == 0 {
		return insufficientResources
	}

	if !ignoreResources.Has(v1.ResourceCPU.String()) && podRequest.MilliCPU > (nodeInfo.Allocatable.MilliCPU-nodeInfo.Requested.MilliCPU) {
		insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
			ResourceName: v1.ResourceCPU,
			Reason:       "Insufficient cpu",
			Requested:    podRequest.MilliCPU,
			Used:         nodeInfo.Requested.MilliCPU,
			Capacity:     nodeInfo.Allocatable.MilliCPU,
		})
	}

	if !ignoreResources.Has(v1.ResourceMemory.String()) && podRequest.Memory > (nodeInfo.Allocatable.Memory-nodeInfo.Requested.Memory) {
		insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
			ResourceName: v1.ResourceMemory,
			Reason:       "Insufficient memory",
			Requested:    podRequest.Memory,
			Used:         nodeInfo.Requested.Memory,
			Capacity:     nodeInfo.Allocatable.Memory,
		})
	}

	if podRequest.EphemeralStorage > (nodeInfo.Allocatable.EphemeralStorage - nodeInfo.Requested.EphemeralStorage) {
		insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
			ResourceName: v1.ResourceEphemeralStorage,
			Reason:       "Insufficient ephemeral-storage",
			Requested:    podRequest.EphemeralStorage,
			Used:         nodeInfo.Requested.EphemeralStorage,
			Capacity:     nodeInfo.Allocatable.EphemeralStorage,
		})
	}

	for rName, rQuant := range podRequest.ScalarResources {
		// Skip in case request quantity is zero
		if rQuant == 0 {
			continue
		}

		if v1helper.IsExtendedResourceName(rName) {
			// If this resource is one of the extended resources that should be ignored, we will skip checking it.
			// rName is guaranteed to have a slash due to API validation.
			var rNamePrefix string
			if ignoredResourceGroups.Len() > 0 {
				rNamePrefix = strings.Split(string(rName), "/")[0]
			}
			if ignoredExtendedResources.Has(string(rName)) || ignoredResourceGroups.Has(rNamePrefix) {
				continue
			}
		}

		if rQuant > (nodeInfo.Allocatable.ScalarResources[rName] - nodeInfo.Requested.ScalarResources[rName]) {
			insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
				ResourceName: rName,
				Reason:       fmt.Sprintf("Insufficient %v", rName),
				Requested:    podRequest.ScalarResources[rName],
				Used:         nodeInfo.Requested.ScalarResources[rName],
				Capacity:     nodeInfo.Allocatable.ScalarResources[rName],
			})
		}
	}

	return insufficientResources
}

func fitsRequestWithTemporal(requested map[string]*UsageTemplate, forecasts map[string]*UsageTemplate, nodeInfo *framework.NodeInfo) []noderesources.InsufficientResource {
	insufficientResources := []noderesources.InsufficientResource{}

	for resource, templates := range forecasts {

		for h, value := range templates.weekDayHour {
			total := int64(math.Round(float64(value)))

			if resource == v1.ResourceCPU.String() && total > nodeInfo.Allocatable.MilliCPU {

				requestedResource := requested[resource]
				requestedVal := int64(math.Round(float64(requestedResource.weekDayHour[h])))
				insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
					ResourceName: v1.ResourceCPU,
					Reason:       fmt.Sprintf("Insufficient cpu at Weekday hour: %d", h),
					Requested:    requestedVal,
					Used:         total - requestedVal,
					Capacity:     nodeInfo.Allocatable.MilliCPU,
				})

			} else if resource == v1.ResourceMemory.String() && total > nodeInfo.Allocatable.Memory {

				requestedResource := requested[resource]
				requestedVal := int64(math.Round(float64(requestedResource.weekDayHour[h])))
				insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
					ResourceName: v1.ResourceMemory,
					Reason:       fmt.Sprintf("Insufficient memory at Weekday hour: %d", h),
					Requested:    requestedVal,
					Used:         total - requestedVal,
					Capacity:     nodeInfo.Allocatable.Memory,
				})

			}
		}

		for h, value := range templates.weekendHour {
			total := int64(math.Round(float64(value)))

			if resource == v1.ResourceCPU.String() && total > nodeInfo.Allocatable.MilliCPU {

				requestedResource := requested[resource]
				requestedVal := int64(math.Round(float64(requestedResource.weekendHour[h])))
				insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
					ResourceName: v1.ResourceCPU,
					Reason:       fmt.Sprintf("Insufficient cpu at Weekend hour: %d", h),
					Requested:    requestedVal,
					Used:         total - requestedVal,
					Capacity:     nodeInfo.Allocatable.MilliCPU,
				})

			} else if resource == v1.ResourceMemory.String() && total > nodeInfo.Allocatable.Memory {

				requestedResource := requested[resource]
				requestedVal := int64(math.Round(float64(requestedResource.weekDayHour[h])))
				insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
					ResourceName: v1.ResourceMemory,
					Reason:       fmt.Sprintf("Insufficient memory at Weekend hour: %d", h),
					Requested:    requestedVal,
					Used:         total - requestedVal,
					Capacity:     nodeInfo.Allocatable.Memory,
				})

			}
		}
	}

	return insufficientResources

}

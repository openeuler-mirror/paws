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


func computePodResourceRequest(pod *v1.Pod) *preFilterState {
	result := &preFilterState{}
	// 处理容器资源请求
	for _, container := range pod.Spec.Containers {
		result.Add(container.Resources.Requests)
	}

	// 处理初始化容器的最大资源请求
	for _, container := range pod.Spec.InitContainers {
		result.SetMaxResource(container.Resources.Requests)
	}

	if pod.Spec.Overhead != nil {
		result.Add(pod.Spec.Overhead)
	}
	return result
}

func fitsRequest(podRequest *preFilterState, nodeInfo *framework.NodeInfo, ignoreResources, ignoredExtendedResources, ignoredResourceGroups sets.String) []noderesources.InsufficientResource {
	insufficientResources := make([]noderesources.InsufficientResource, 0, 4)

	// 检查pod数量是否超出节点可容纳的最大数量
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

	// 如果请求资源为空，直接返回
	if podRequest.MilliCPU == 0 && podRequest.Memory == 0 && podRequest.EphemeralStorage == 0 && len(podRequest.ScalarResources) == 0 {
		return insufficientResources
	}

	// 检查资源请求是否超出节点分配的可用资源
	checkResource := func(resourceName string, requested int64, nodeAllocatable int64, nodeRequested int64, ignore bool) {
		if !ignore && requested > (nodeAllocatable-nodeRequested) {
			insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
				ResourceName: resourceName,
				Reason:       fmt.Sprintf("Insufficient %v", resourceName),
				Requested:    requested,
				Used:         nodeRequested,
				Capacity:     nodeAllocatable,
			})
		}
	}

	checkResource(v1.ResourceCPU.String(), podRequest.MilliCPU, nodeInfo.Allocatable.MilliCPU, nodeInfo.Requested.MilliCPU, ignoreResources.Has(v1.ResourceCPU.String()))
	checkResource(v1.ResourceMemory.String(), podRequest.Memory, nodeInfo.Allocatable.Memory, nodeInfo.Requested.Memory, ignoreResources.Has(v1.ResourceMemory.String()))
	checkResource(v1.ResourceEphemeralStorage.String(), podRequest.EphemeralStorage, nodeInfo.Allocatable.EphemeralStorage, nodeInfo.Requested.EphemeralStorage, false)

	// 检查标量资源请求
	for rName, rQuant := range podRequest.ScalarResources {
		if rQuant == 0 {
			continue
		}

		// 如果是扩展资源且需要忽略，跳过检查
		if v1helper.IsExtendedResourceName(rName) {
			if ignoredExtendedResources.Has(string(rName)) || ignoredResourceGroups.Has(strings.Split(string(rName), "/")[0]) {
				continue
			}
		}

		// 检查资源是否足够
		if rQuant > (nodeInfo.Allocatable.ScalarResources[rName] - nodeInfo.Requested.ScalarResources[rName]) {
			insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
				ResourceName: rName,
				Reason:       fmt.Sprintf("Insufficient %v", rName),
				Requested:    rQuant,
				Used:         nodeInfo.Requested.ScalarResources[rName],
				Capacity:     nodeInfo.Allocatable.ScalarResources[rName],
			})
		}
	}

	return insufficientResources
}

func fitsRequestWithTemporal(requested map[string]*UsageTemplate, forecasts map[string]*UsageTemplate, nodeInfo *framework.NodeInfo) []noderesources.InsufficientResource {
	insufficientResources := []noderesources.InsufficientResource{}

	checkTemporalResource := func(resource string, templates *UsageTemplate, requested map[string]*UsageTemplate) {
		for _, timePeriod := range [][]int{templates.weekDayHour, templates.weekendHour} {
			for h, value := range timePeriod {
				total := int64(math.Round(float64(value)))

				if resource == v1.ResourceCPU.String() && total > nodeInfo.Allocatable.MilliCPU {
					requestedResource := requested[resource]
					requestedVal := int64(math.Round(float64(requestedResource.weekDayHour[h])))
					insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
						ResourceName: v1.ResourceCPU,
						Reason:       fmt.Sprintf("Insufficient cpu at hour: %d", h),
						Requested:    requestedVal,
						Used:         total - requestedVal,
						Capacity:     nodeInfo.Allocatable.MilliCPU,
					})
				} else if resource == v1.ResourceMemory.String() && total > nodeInfo.Allocatable.Memory {
					requestedResource := requested[resource]
					requestedVal := int64(math.Round(float64(requestedResource.weekDayHour[h])))
					insufficientResources = append(insufficientResources, noderesources.InsufficientResource{
						ResourceName: v1.ResourceMemory,
						Reason:       fmt.Sprintf("Insufficient memory at hour: %d", h),
						Requested:    requestedVal,
						Used:         total - requestedVal,
						Capacity:     nodeInfo.Allocatable.Memory,
					})
				}
			}
		}
	}

	// 遍历每种资源的模板
	for resource, templates := range forecasts {
		checkTemporalResource(resource, templates, requested)
	}

	return insufficientResources
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


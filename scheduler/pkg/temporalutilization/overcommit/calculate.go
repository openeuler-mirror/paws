package overcommit

import (
	"fmt"
	"math"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

func getNodeAllocatableResource(nodeInfo *framework.NodeInfo, resourceName string) (int64, error) {
	switch resourceName {
	case v1.ResourceCPU.String():
		return nodeInfo.Allocatable.MilliCPU, nil
	case v1.ResourceMemory.String():
		return nodeInfo.Allocatable.Memory, nil
	case v1.ResourceEphemeralStorage.String():
		return nodeInfo.Allocatable.EphemeralStorage, nil
	default:
		return 0, fmt.Errorf("can not get resource %v, not supported", resourceName)
	}
}

// convertToQuantity follow the kubernetes framework convension quantity
func convertToQuantity(resourceName string, value int64) (resource.Quantity, error) {
	switch resourceName {
	case v1.ResourceCPU.String():
		return *resource.NewMilliQuantity(value, resource.DecimalSI), nil
	case v1.ResourceMemory.String():
		return *resource.NewQuantity(value, resource.BinarySI), nil
	case v1.ResourceEphemeralStorage.String():
		return *resource.NewQuantity(value, resource.BinarySI), nil
	default:
		return resource.Quantity{}, fmt.Errorf("can not convert resource %v, not supported", resourceName)
	}
}

func CalculateOvercommitResources(nodeInfo *framework.NodeInfo, supportedOvercommitResource map[string]string) (v1.ResourceList, error) {
	results := v1.ResourceList{}
	for resourceName, annotation := range supportedOvercommitResource {
		v, ok := nodeInfo.Node().Annotations[annotation]
		if !ok {
			continue
		}

		ratio, err := strconv.ParseFloat(v, 32)
		if err != nil {
			return results, err
		}

		// not allow negative ratio
		if ratio < 0.0 {
			return results, fmt.Errorf("invalid ratio, got %v", ratio)
		}

		allocatable, err := getNodeAllocatableResource(nodeInfo, resourceName)
		if err != nil {
			return results, err
		}

		value := int64(math.Round(float64(allocatable) * ratio))

		quant, err := convertToQuantity(resourceName, value)
		if err != nil {
			return results, err
		}

		results[v1.ResourceName(resourceName)] = quant
	}

	return results, nil
}

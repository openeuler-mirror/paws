package overcommit

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
// Description: This file is used for overcommitment calculation

import (
	"fmt"
	"math"
	"strconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/kubernetes/pkg/scheduler/framework"
)

// getNodeAllocatableResource retrieves the allocatable resource from the node based on the resource name.
func getNodeAllocatableResource(nodeInfo *framework.NodeInfo, resourceName string) (int64, error) {
	switch resourceName {
	case v1.ResourceCPU.String():
		return nodeInfo.Allocatable.MilliCPU, nil
	case v1.ResourceMemory.String():
		return nodeInfo.Allocatable.Memory, nil
	case v1.ResourceEphemeralStorage.String():
		return nodeInfo.Allocatable.EphemeralStorage, nil
	default:
		return 0, fmt.Errorf("resource %v not supported", resourceName)
	}
}

// convertToQuantity converts the resource value to a Kubernetes Quantity based on the resource name.
func convertToQuantity(resourceName string, value int64) (resource.Quantity, error) {
	switch resourceName {
	case v1.ResourceCPU.String():
		return *resource.NewMilliQuantity(value, resource.DecimalSI), nil
	case v1.ResourceMemory.String():
		return *resource.NewQuantity(value, resource.BinarySI), nil
	case v1.ResourceEphemeralStorage.String():
		return *resource.NewQuantity(value, resource.BinarySI), nil
	default:
		return resource.Quantity{}, fmt.Errorf("cannot convert resource %v, not supported", resourceName)
	}
}

// CalculateOvercommitResources calculates the overcommitted resources for the node based on the given ratios in annotations.
func CalculateOvercommitResources(nodeInfo *framework.NodeInfo, supportedOvercommitResource map[string]string) (v1.ResourceList, error) {
	results := v1.ResourceList{}
	var errors []error // Collect errors for all resources

	for resourceName, annotation := range supportedOvercommitResource {
		v, ok := nodeInfo.Node().Annotations[annotation]
		if !ok {
			continue
		}

		// Parse the ratio from the annotation
		ratio, err := strconv.ParseFloat(v, 64) // Using 64-bit float for better precision
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to parse ratio for resource %v: %v", resourceName, err))
			continue
		}

		// Ensure ratio is non-negative
		if ratio < 0.0 {
			errors = append(errors, fmt.Errorf("invalid ratio for resource %v: got %v", resourceName, ratio))
			continue
		}

		// Get allocatable resource for the node
		allocatable, err := getNodeAllocatableResource(nodeInfo, resourceName)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to get allocatable resource for %v: %v", resourceName, err))
			continue
		}

		// Calculate the overcommitted value
		value := int64(math.Round(float64(allocatable) * ratio))

		// Convert the calculated value to Quantity
		quant, err := convertToQuantity(resourceName, value)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to convert resource %v to quantity: %v", resourceName, err))
			continue
		}

		results[v1.ResourceName(resourceName)] = quant
	}

	// If there are errors, return them
	if len(errors) > 0 {
		return results, fmt.Errorf("encountered errors: %v", errors)
	}

	return results, nil
}

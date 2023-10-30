package temporalutilization

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/v1/resource"
	v1qos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"k8s.io/kubernetes/pkg/scheduler/framework/plugins/noderesources"
	sutil "k8s.io/kubernetes/pkg/scheduler/util"
)

func sameUtilizationByHour(milliValue float32) *UsageTemplate {
	results := UsageTemplate{
		weekDayHour: make(map[int16]float32),
		weekendHour: make(map[int16]float32),
	}

	for i := 0; i < NumHoursInADay; i++ {
		results.weekDayHour[int16(i)] = milliValue
		results.weekendHour[int16(i)] = milliValue
	}

	return &results
}

func getUsageTemplatesByPod(utMgr *UsageTemplateManager, namespace string, podName string, targetResources []string) (map[string]*UsageTemplate, error) {
	pod, err := utMgr.podLister.Pods(namespace).Get(podName)
	if err != nil {
		return nil, err
	}

	_, ut := utMgr.GetUsageTemplate(pod)
	podUsages := make(map[string]*UsageTemplate)

	// assume utilization by class when there is no CRD or we are not using it
	for _, res := range targetResources {
		var err error

		if ut == nil || !ut.Spec.Enabled {
			podUsages[res], err = assumeUsageByClass(pod, res)
		} else {
			if ut.Status.HistoricalUsage != nil {
				podUsages[res], err = extractUsageFromCRD(ut, res, time.Now().UTC().Hour())
			} else {
				// we do not have any historical usage yet
				podUsages[res], err = assumeUsageByClass(pod, res)
			}
		}

		if err != nil {
			klog.ErrorS(err, "unable to obtain usage for pod",
				"resource", res,
				"pod", pod.Name,
				"namespace", pod.Namespace)
			return nil, err
		}
	}

	return podUsages, nil
}

func sumPodUsageByNode(cachePods []NamespacedPod, utMgr *UsageTemplateManager, targetResources []string) (map[string]*UsageTemplate, error) {
	results := make(map[string]*UsageTemplate)

	for _, cachePod := range cachePods {

		podUsages, err := getUsageTemplatesByPod(utMgr, cachePod.Namespace, cachePod.Name, targetResources)
		if err != nil {
			return results, err
		}

		for resource, usageTemplate := range podUsages {
			if _, ok := results[resource]; !ok {
				results[resource] = &UsageTemplate{
					resource:    resource,
					unit:        DefaultResourceUnitMap[resource],
					weekDayHour: make(map[int16]float32),
					weekendHour: make(map[int16]float32),
				}
			}

			if usageTemplate == nil {
				klog.Warningf("empty usage template for pod: %s | resource: %s", cachePod.Name, resource)
				continue
			}

			for hour, value := range usageTemplate.weekDayHour {
				if _, ok := results[resource].weekDayHour[hour]; !ok {
					results[resource].weekDayHour[hour] = value
				} else {
					results[resource].weekDayHour[hour] += value
				}
			}

			for hour, value := range usageTemplate.weekendHour {
				if _, ok := results[resource].weekendHour[hour]; !ok {
					results[resource].weekendHour[hour] = value
				} else {
					results[resource].weekendHour[hour] += value
				}
			}
		}

	}

	return results, nil
}

func sumUsageByHour(a, b map[string]*UsageTemplate) map[string]*UsageTemplate {
	results := make(map[string]*UsageTemplate)

	// what is in a, might not be in b
	// and vice versa
	for resource, template := range a {
		results[resource] = template
	}

	for resource, template := range b {
		if out, ok := results[resource]; ok {
			for hour, bvalue := range template.weekDayHour {
				if aValue, okh := out.weekDayHour[hour]; okh {
					out.weekDayHour[hour] = aValue + bvalue
				} else {
					out.weekDayHour[hour] = bvalue
				}
			}
		} else {
			// take b if not exists
			results[resource] = template
		}
	}

	return results
}

func getDefaultResourceValue(resourceName string) (int64, error) {
	switch resourceName {
	case v1.ResourceCPU.String():
		return sutil.DefaultMilliCPURequest, nil
	case v1.ResourceMemory.String():
		return sutil.DefaultMemoryRequest, nil
	default:
		return 0, fmt.Errorf("unsupported resource %s", resourceName)
	}
}

func getResourceValue(resourceName string, resourceList v1.ResourceList) (int64, error) {
	switch resourceName {
	case v1.ResourceCPU.String():
		return resourceList.Cpu().MilliValue(), nil
	case v1.ResourceMemory.String():
		return resourceList.Memory().MilliValue(), nil
	default:
		return 0, fmt.Errorf("unsupported resource %s", resourceName)
	}
}

func assumeUsageByClass(p *v1.Pod, resourceName string) (*UsageTemplate, error) {

	pClass := v1qos.GetPodQOS(p)

	// for best efforts, no requests or limits
	if pClass == v1.PodQOSBestEffort {
		v, err := getDefaultResourceValue(resourceName)
		if err != nil {
			return nil, err
		}
		return sameUtilizationByHour(float32(v)), nil
	}

	reqs, limits := resource.PodRequestsAndLimits(p)

	if pClass == v1.PodQOSGuaranteed {
		v, err := getResourceValue(resourceName, limits)
		if err != nil {
			return nil, err
		}
		return sameUtilizationByHour(float32(v)), nil
	}

	v, err := getResourceValue(resourceName, reqs)
	if err != nil {
		return nil, err
	}
	return sameUtilizationByHour(float32(v)), nil
}

func extractUsageFromCRD(ut *v1alpha1.UsageTemplate, resourceName string, currentHour int) (*UsageTemplate, error) {
	results := &UsageTemplate{
		resource:    resourceName,
		weekDayHour: make(map[int16]float32),
		weekendHour: make(map[int16]float32),
	}

	if ut == nil || ut.Status.HistoricalUsage == nil {
		return nil, fmt.Errorf("no usage template historical usage")
	}

	historicalUsage := ut.Status.HistoricalUsage

	offset := 0

	// when an app is not longrunning, we off set the hour from current hour
	if !ut.Status.IsLongRunning {
		offset = currentHour
	}

	for _, item := range historicalUsage.Items {
		if item.Resource != resourceName {
			continue
		}

		for _, usage := range item.Usages {
			v, err := strconv.ParseFloat(usage.Value, 64)
			if err != nil {
				klog.ErrorS(err, "cannot parse float", "value", usage.Value)
				continue
			}

			offsetHour := math.Mod(float64(offset)+float64(usage.Hour), NumHoursInADay)

			if usage.IsWeekday {
				results.weekDayHour[int16(offsetHour)] = float32(v)
			} else {
				results.weekendHour[int16(offsetHour)] = float32(v)
			}
		}
	}

	// if the app is not long running, and it shows up in weekend,
	// but we are now in weekdays, we assume that it also has the same usage in weekdays
	// copy the hourly usage over from weekday -> weekend
	// and vice versa to make sure it covers both weekday and weekend
	// NOTE: The copying only happens if no historical usages are present for the hour.
	if !ut.Status.IsLongRunning {

		// prepare the original weekend hour before copying
		oldWeekendHour := map[int16]float32{}
		for k, v := range results.weekendHour {
			oldWeekendHour[k] = v
		}

		// copy from weekday to weekend
		for k, v := range results.weekDayHour {
			if _, ok := results.weekendHour[k]; !ok {
				results.weekendHour[k] = v
			}
		}

		// copy from weekend to weekday
		for k, v := range oldWeekendHour {
			if _, ok := results.weekDayHour[k]; !ok {
				results.weekDayHour[k] = v
			}
		}
	}

	klog.V(6).InfoS("UsageTemplate Extracted", "UT", klog.KObj(ut),
		"Resource", results.resource,
		"Weekday Hour", results.weekDayHour,
		"Weekend Hour", results.weekendHour)

	return results, nil
}

func obtainForecasts(utMgr *UsageTemplateManager, nodeInfo *framework.NodeInfo, nodeName string, pod *v1.Pod, supportedTargetResources []string) (map[string]*UsageTemplate, string, error) {

	pods := utMgr.GetNodePods(nodeName)

	pods = append(pods, NamespacedPod{Namespace: pod.Namespace, Name: pod.Name})

	// let's calculate the forecast and see what we get
	forecasts, err := sumPodUsageByNode(pods, utMgr, supportedTargetResources)
	if err != nil {
		return nil, fmt.Sprintf("Summing Pod Usages for node %q: %v", nodeName, err), err
	}

	for k, v := range forecasts {
		if v != nil {
			klog.V(6).InfoS("Forecast", "Node", klog.KObj(nodeInfo.Node()), "Resource", k, "UsageTemplate: WeekDay", v.weekDayHour, "UsageTemplate: WeekEnd", v.weekendHour)
		}
	}

	return forecasts, "", nil
}

func checkInsufficientResources(insufficientResources []noderesources.InsufficientResource) *framework.Status {
	if len(insufficientResources) != 0 {
		// We will keep all failure reasons.
		failureReasons := make([]string, 0, len(insufficientResources))
		for i := range insufficientResources {
			failureReasons = append(failureReasons, insufficientResources[i].Reason)
		}
		return framework.NewStatus(framework.Unschedulable, failureReasons...)
	}
	return nil
}

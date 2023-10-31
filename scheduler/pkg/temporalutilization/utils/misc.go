package utils

import (
	"time"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func BeforeUTC(a, b time.Time) bool {
	return a.UTC().Before(b.UTC())
}

func AfterUTC(a, b time.Time) bool {
	return a.UTC().After(b.UTC())
}

func GetUsageTemplateLabel(pod *v1.Pod) string {
	return pod.Labels[v1alpha1.UsageTemplateLabelIdentifier]
}

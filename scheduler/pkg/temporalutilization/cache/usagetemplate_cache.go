package cache

import (
	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	"k8s.io/client-go/tools/record"
)

type UsageTemplateCache struct {
	Object           *v1alpha1.UsageTemplate
	ObjectGeneration int64
	Recorder         record.EventRecorder
}

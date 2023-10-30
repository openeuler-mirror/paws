package temporalutilization

import (
	"context"
	"math"
	"time"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
)

type UsageTemplate struct {
	resource string
	// For Memory, it is the bytes. i.e. unit = 'bytes'
	// For CPU, it is the MilliValue. i.e. unit = 'millicore'
	unit string
	// weekDayHour is a map of hour to configured Percentile of Usage value in weekdays.
	// index by 0
	weekDayHour map[int16]float32
	// weekDayHour is a map of hour to configured Percentile of Usage value in weekend.
	// index by 0
	weekendHour map[int16]float32
}

func (u *UsageTemplate) MaxUsage() float32 {
	v := float32(-math.MaxFloat32)
	for _, value := range u.weekDayHour {
		if value > v {
			v = value
		}
	}

	for _, value := range u.weekendHour {
		if value > v {
			v = value
		}
	}
	return v
}

type NamespacedPod struct {
	Namespace string
	Name      string
}

type QueuedUsageTemplate struct {
	*v1alpha1.UsageTemplate
	Context context.Context
	// The last successful evaluation timestamp
	LastEvaluated time.Time
	// Assumed next evaluation timestamp, set by last evaluation cycle
	NextEvaluationTime time.Time
	// Number of evaluation done
	Counts int
}

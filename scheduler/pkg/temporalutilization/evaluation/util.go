package evaluation

import (
	"fmt"
	"math"
	"time"

	"gitee.com/openeuler/paws/scheduler/apis/scheduling/v1alpha1"
	tutils "gitee.com/openeuler/paws/scheduler/pkg/temporalutilization/utils"
	"github.com/prometheus/common/model"
)

const (
	containerPromMetricLabel = "container"
)

// GetWeekDifferenceUTC returns the week difference from end to start, given startTime, and endTime
func GetWeekDifferenceUTC(start, end time.Time) int {
	diff := end.UTC().Sub(start.UTC())
	return int(math.Round(diff.Hours() / (24.0 * 7.0)))
}

func AddSampleByWeightedWeekUTC(h *dateTimeEstimator, maxWeek, weeksDiff int, t time.Time, givenHour int, value float64) {
	weight := 1
	t = t.UTC()
	hour := givenHour
	// we should re-calculate the weights so that the samples that are weeks away have less weight
	if maxWeek > 0 {
		// e.g. maxWeek is 3
		// case 1. current sample is within the same week as now, will receive a weight of 4 (3-0+1)
		// case 2. current sample is further away, 3 weeks ago, will receive a weight of 1 (3-3+1)
		// case 3. current sample is two weeks ago, will receive a weight of 2 (3-2+1)
		weight = maxWeek - weeksDiff + 1
	}

	// Sunday is 0, Saturday is 6
	isWeekday := int(t.Weekday()) >= 1 && int(t.Weekday()) <= 5
	if !isWeekday {
		hour += 24
	}
	// Add the sample using the similar idea of load signal area , unitTime*Value from AutoPilot
	// only that we assume a unit time is how far off we are, for example:
	// we have seven values: [[weekWeight: 1, value: 2], [weekWeight: 1, value: 2], [weekWeight: 1, value: 2],
	// 						[weekWeight: 1, value: 2], [weekWeight: 1, value: 2], [weekWeight: 2, value: 2],
	//						[weekWeight: 3, value: 10]]
	// a normal exp historgram P95 that uses count would give us value near the boundary of 2.
	// a simple week weighted exp histogram would also give us value near the boundary of 2,
	// the weekValueWeighted Load exp histogram would give us a bucket value somewhere before 10,
	// to better accomodate quick changes
	h.addSample(hour, value, float64(weight)*value, t)
}

func FindMaxWeekAndCheckIsLongRunning(values model.Value, now time.Time) (int, bool, error) {
	containersMaxweek := make(map[string]int)
	isLongRunning := false
	maxWeek := 0

	// loop through each series
	// 1. get its duration
	// 2. calculate the oldest (max) week
	var maxDuration time.Duration
	switch values := values.(type) {
	case model.Matrix:
		for _, series := range values {
			containerName, ok := series.Metric[containerPromMetricLabel]
			if !ok {
				continue
			}
			startTime := now
			var endTime time.Time
			for _, v := range series.Values {
				sampleTime := v.Timestamp.Time()
				weekDiff := GetWeekDifferenceUTC(sampleTime, now)
				maxWeek = tutils.Max(maxWeek, weekDiff)
				if tutils.BeforeUTC(sampleTime, startTime) {
					startTime = sampleTime
				}

				if tutils.AfterUTC(sampleTime, endTime) {
					endTime = sampleTime
				}

				duration := endTime.Sub(startTime)
				if duration > maxDuration {
					maxDuration = duration
				}
				containersMaxweek[string(containerName)] = maxWeek
				log.V(6).Info("Container: %v, Ran for: %v, MaxDuration across containers: %v\n", containerName, duration, maxDuration)
			}
		}
	default:
		return 0, false, fmt.Errorf("unsupported model type: %v", values.Type().String())
	}

	// double check that we should have more than one type of container name here
	if len(containersMaxweek) > 1 {
		containerNames := make([]string, 0)
		for k, _ := range containersMaxweek {
			containerNames = append(containerNames, k)
		}
		return 0, false, fmt.Errorf("expected one container only, got: %v", containerNames)
	} else if len(containersMaxweek) == 0 {
		// no containers ?
		return 0, false, fmt.Errorf("expected at least one container, got zero")
	}

	if maxDuration > time.Hour*24 {
		isLongRunning = true
	}

	return maxWeek, isLongRunning, nil
}

func AddWeightedSampleByWeek(h *dateTimeEstimator, maxWeek int, values model.Value, now time.Time) error {
	switch values := values.(type) {
	case model.Matrix:
		for _, series := range values {
			for _, vv := range series.Values {
				sampleTime := vv.Timestamp.Time()
				weeksDiff := GetWeekDifferenceUTC(now, sampleTime)
				AddSampleByWeightedWeekUTC(h, maxWeek, weeksDiff, sampleTime, sampleTime.Hour(), float64(vv.Value))
			}
		}

	default:
		err := fmt.Errorf("expected Matrix type, but got %v", values.Type())
		log.Error(err, "unable to build histogram")
		return err
	}

	return nil
}

func AddShiftedWeightedSampleByWeek(h *dateTimeEstimator, maxWeek int, values model.Value, now time.Time) error {
	switch values := values.(type) {
	case model.Matrix:
		for _, series := range values {
			// first because we need to shift, that means we need to find the min time of this series
			seriesMinTime := now

			// find min time
			for _, vv := range series.Values {
				sampleTime := vv.Timestamp.Time()
				if tutils.BeforeUTC(sampleTime, seriesMinTime) {
					seriesMinTime = sampleTime
				}
			}

			// now add the sample
			// but take away the time in order to get the right hour
			for _, vv := range series.Values {
				sampleTime := vv.Timestamp.Time()
				weeksDiff := GetWeekDifferenceUTC(now, sampleTime)
				diff := sampleTime.Sub(seriesMinTime)
				givenHour := int(math.Round(diff.Hours()))
				if givenHour < 0 {
					return fmt.Errorf("unexpected hour differences, sample time: %v, min time: %v, diff: %v", sampleTime, seriesMinTime, diff)
				}
				AddSampleByWeightedWeekUTC(h, maxWeek, weeksDiff, sampleTime, givenHour, float64(vv.Value))
			}

		}

	default:
		err := fmt.Errorf("expected Matrix type, but got %v", values.Type())
		log.Error(err, "unable to build histogram")
		return err
	}

	return nil
}

func GetNamespacedName(ut *v1alpha1.UsageTemplate) string {
	return fmt.Sprintf("%s/%s", ut.Namespace, ut.Name)
}

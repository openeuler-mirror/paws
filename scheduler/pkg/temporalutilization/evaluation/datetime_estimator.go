package evaluation

import (
	"time"

	kvpa "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/recommender/util"
)

const (
	// DefaultHistogramBucketSizeGrowth is the default value for HistogramBucketSizeGrowth.
	DefaultHistogramBucketSizeGrowth = 0.05 // Make each bucket 5% larger than the previous one.
	// minSampleWeight is the minimal weight of any sample (prior to including decaying factor)
	minSampleWeight = 0.1
	// epsilon is the minimal weight kept in histograms, it should be small enough that old samples
	// (just inside MemoryAggregationWindowLength) added with minSampleWeight are still kept
	epsilon = 0.001 * minSampleWeight
	// DefaultCPUHistogramDecayHalfLife is the default value for CPUHistogramDecayHalfLife.
	// CPU usage sample to lose half of its weight.
	// In our current implementation, it is a month
	DefaultCPUHistogramDecayHalfLife = time.Hour * 24 * 30
)

// TODO: to identify whether Saturday and Sunday have differences
type dateTimeEstimator struct {
	// Histograms contain time of day histogram for each hour, and
	// should have length of 48, where the first 24 hours are weekdays,
	// the last 24 hours are of weekends
	Histograms []hourEstimator
}

type hourEstimator struct {
	// Hour is index by 0
	Hour int
	kvpa.Histogram
	IsWeekday bool
}

func makeExpHistogram() (kvpa.Histogram, error) {
	// max of 1000.0 cores, with first bucket at 0.1
	// growth rate of 1.05
	opts, err := kvpa.NewExponentialHistogramOptions(1000.0, 0.1, 1.0+DefaultHistogramBucketSizeGrowth, epsilon)
	if err != nil {
		return nil, err
	}
	return kvpa.NewDecayingHistogram(opts, DefaultCPUHistogramDecayHalfLife), nil
}

func NewDateTimeEstimator() (*dateTimeEstimator, error) {
	requireNum := 48
	de := &dateTimeEstimator{
		// First 24 is weekday histograms
		// the last 24 is weekend histograms
		Histograms: make([]hourEstimator, requireNum),
	}

	for i := 0; i < requireNum; i++ {
		kh, err := makeExpHistogram()
		if err != nil {
			return nil, err
		}
		weekday := true
		if i >= 24 {
			weekday = false
		}

		h := hourEstimator{
			Hour:      i,
			Histogram: kh,
			IsWeekday: weekday,
		}
		de.Histograms[i] = h
	}

	return de, nil
}

func (de *dateTimeEstimator) addSample(hour int, v float64, weight float64, time time.Time) {
	de.Histograms[hour].AddSample(v, weight, time)
}

// IsLongRunning checks whether the application has run longer than 24 hours
func (de *dateTimeEstimator) IsLongRunning() bool {
	// 1. if all its weekdays hour are not empty, we say it is long running.
	// 2. if all its weekends hour are not empty, we say it is long running.

	weekdaysEmpty := false
	n := len(de.Histograms)
	for i := 0; i < n/2; i++ {
		if de.Histograms[i].IsEmpty() {
			weekdaysEmpty = true
		}
	}

	weekendsEmpty := false
	for i := n / 2; i < n; i++ {
		if de.Histograms[i].IsEmpty() {
			weekendsEmpty = true
		}
	}

	return !(weekdaysEmpty && weekendsEmpty)
}

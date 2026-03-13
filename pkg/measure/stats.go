package measure

import (
	"math"
	"time"
)

// Mean returns the arithmetic mean of vals. Returns 0 for empty slices.
func Mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

// StdDev returns the population standard deviation of vals.
func StdDev(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := Mean(vals)
	var sqDevSum float64
	for _, v := range vals {
		d := v - m
		sqDevSum += d * d
	}
	return math.Sqrt(sqDevSum / float64(len(vals)))
}

// MinMax returns the minimum and maximum of vals.
// Returns (0, 0) for empty slices.
func MinMax(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	min, max := vals[0], vals[0]
	for _, v := range vals[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

// DurationStats computes min, max, mean, and standard deviation for a slice of durations.
func DurationStats(pings []time.Duration) (min, max, mean, stddev time.Duration) {
	if len(pings) == 0 {
		return 0, 0, 0, 0
	}

	vals := make([]float64, len(pings))
	for i, p := range pings {
		vals[i] = float64(p)
	}

	fmin, fmax := MinMax(vals)
	return time.Duration(fmin), time.Duration(fmax), time.Duration(Mean(vals)), time.Duration(StdDev(vals))
}

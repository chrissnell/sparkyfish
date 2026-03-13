package measure

import (
	"math"
	"testing"
	"time"
)

func approxEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestMean(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5.0}, 5.0},
		{"multiple", []float64{1, 2, 3, 4, 5}, 3.0},
		{"negative", []float64{-2, 0, 2}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Mean(tt.vals)
			if !approxEqual(got, tt.want, 1e-9) {
				t.Errorf("Mean(%v) = %v, want %v", tt.vals, got, tt.want)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []struct {
		name string
		vals []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{5.0}, 0},
		{"uniform", []float64{3, 3, 3}, 0},
		{"known", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StdDev(tt.vals)
			if !approxEqual(got, tt.want, 1e-9) {
				t.Errorf("StdDev(%v) = %v, want %v", tt.vals, got, tt.want)
			}
		})
	}
}

func TestMinMax(t *testing.T) {
	tests := []struct {
		name     string
		vals     []float64
		wantMin  float64
		wantMax  float64
	}{
		{"empty", nil, 0, 0},
		{"single", []float64{7}, 7, 7},
		{"ordered", []float64{1, 2, 3}, 1, 3},
		{"reversed", []float64{3, 2, 1}, 1, 3},
		{"negative", []float64{-5, 0, 5}, -5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMin, gotMax := MinMax(tt.vals)
			if gotMin != tt.wantMin || gotMax != tt.wantMax {
				t.Errorf("MinMax(%v) = (%v, %v), want (%v, %v)", tt.vals, gotMin, gotMax, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDurationStats(t *testing.T) {
	pings := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
	}

	min, max, mean, stddev := DurationStats(pings)

	if min != 10*time.Millisecond {
		t.Errorf("min = %v, want 10ms", min)
	}
	if max != 30*time.Millisecond {
		t.Errorf("max = %v, want 30ms", max)
	}
	if mean != 20*time.Millisecond {
		t.Errorf("mean = %v, want 20ms", mean)
	}

	// stddev of [10, 20, 30] ms = sqrt(200/3) ms ≈ 8.165ms
	wantStddev := time.Duration(math.Sqrt(200.0/3.0) * float64(time.Millisecond))
	tolerance := 100 * time.Microsecond
	if stddev < wantStddev-tolerance || stddev > wantStddev+tolerance {
		t.Errorf("stddev = %v, want ~%v", stddev, wantStddev)
	}
}

func TestDurationStatsEmpty(t *testing.T) {
	min, max, mean, stddev := DurationStats(nil)
	if min != 0 || max != 0 || mean != 0 || stddev != 0 {
		t.Errorf("DurationStats(nil) = (%v, %v, %v, %v), want all zeros", min, max, mean, stddev)
	}
}

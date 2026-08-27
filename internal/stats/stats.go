package stats

import (
	"math"
	"slices"
	"time"
)

// Accumulator collects request outcomes as workers complete them.
type Accumulator struct {
	summary             Summary
	successfulDurations []time.Duration
}

// NewAccumulator creates a streaming benchmark stats collector.
func NewAccumulator(initialCapacity int) *Accumulator {
	return &Accumulator{
		summary: Summary{
			StatusCodes: make(map[int]int),
			Errors:      make(map[string]int),
		},
		successfulDurations: make([]time.Duration, 0, initialCapacity),
	}
}

// Add records one completed request result.
func (a *Accumulator) Add(result RequestResult) {
	a.summary.AttemptedRequests++

	if result.StatusCode > 0 {
		a.summary.StatusCodes[result.StatusCode]++
	}
	if result.Error != nil {
		a.summary.Errors[result.Error.Error()]++
	}
	if !isSuccessfulResult(result) {
		a.summary.FailedRequests++
		return
	}

	a.summary.SuccessfulRequests++
	a.successfulDurations = append(a.successfulDurations, result.Duration)
}

// Finalize calculates latency and throughput for the completed run.
func (a *Accumulator) Finalize(elapsedTime time.Duration) Summary {
	a.summary.ElapsedTime = elapsedTime
	a.summary.SuccessfulLatency = calculateLatencyStats(a.successfulDurations)

	if elapsedTime.Seconds() > 0 {
		a.summary.EstimatedThroughput = float64(a.summary.AttemptedRequests) / elapsedTime.Seconds()
		a.summary.SuccessfulThroughput = float64(a.summary.SuccessfulRequests) / elapsedTime.Seconds()
	}

	return a.summary
}

// CalculateStats processes collected results and computes performance metrics.
func CalculateStats(results []RequestResult, elapsedTime time.Duration) Summary {
	stats := NewAccumulator(len(results))
	for _, result := range results {
		stats.Add(result)
	}

	return stats.Finalize(elapsedTime)
}

func isSuccessfulResult(result RequestResult) bool {
	return result.Error == nil && result.StatusCode >= 200 && result.StatusCode < 400
}

func calculateLatencyStats(durations []time.Duration) LatencyStats {
	if len(durations) == 0 {
		return LatencyStats{}
	}

	slices.Sort(durations)

	var total time.Duration
	for _, duration := range durations {
		total += duration
	}

	return LatencyStats{
		Average: total / time.Duration(len(durations)),
		Minimum: durations[0],
		Maximum: durations[len(durations)-1],
		P50:     percentile(durations, 0.50),
		P90:     percentile(durations, 0.90),
		P95:     percentile(durations, 0.95),
		P99:     percentile(durations, 0.99),
	}
}

func percentile(sortedDurations []time.Duration, percentile float64) time.Duration {
	index := int(math.Ceil(percentile*float64(len(sortedDurations)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sortedDurations) {
		index = len(sortedDurations) - 1
	}

	return sortedDurations[index]
}

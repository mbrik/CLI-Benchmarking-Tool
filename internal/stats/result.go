package stats

import "time"

// RequestResult holds the outcome of one HTTP request execution.
type RequestResult struct {
	Duration   time.Duration
	Error      error
	StatusCode int
}

// LatencyStats describes the latency distribution for successful requests.
type LatencyStats struct {
	Average time.Duration
	Minimum time.Duration
	Maximum time.Duration
	P50     time.Duration
	P90     time.Duration
	P95     time.Duration
	P99     time.Duration
}

// Summary contains the aggregate results of a benchmark run.
type Summary struct {
	AttemptedRequests    int
	SuccessfulRequests   int
	FailedRequests       int
	ElapsedTime          time.Duration
	EstimatedThroughput  float64
	SuccessfulThroughput float64
	SuccessfulLatency    LatencyStats
	StatusCodes          map[int]int
	Errors               map[string]int
}

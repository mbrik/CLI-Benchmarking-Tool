package stats_test

import (
	"io"
	"math"
	"testing"
	"time"

	"github.com/mbrik/CLI-Benchmarking-Tool/internal/stats"
)

func TestCalculateStatsWithErrorsAndFailures(t *testing.T) {
	results := []stats.RequestResult{
		{Duration: 10 * time.Millisecond, StatusCode: 200},
		{Duration: 15 * time.Millisecond, StatusCode: 404},
		{Duration: 20 * time.Millisecond, StatusCode: 500},
		{Duration: 5 * time.Millisecond, Error: io.ErrUnexpectedEOF},
	}

	summary := stats.CalculateStats(results, 50*time.Millisecond)
	if summary.AttemptedRequests != 4 {
		t.Fatalf("expected 4 attempted requests, got %d", summary.AttemptedRequests)
	}
	if summary.SuccessfulRequests != 1 {
		t.Fatalf("expected 1 successful request, got %d", summary.SuccessfulRequests)
	}
	if summary.FailedRequests != 3 {
		t.Fatalf("expected 3 failed requests, got %d", summary.FailedRequests)
	}
	if summary.StatusCodes[200] != 1 || summary.StatusCodes[404] != 1 || summary.StatusCodes[500] != 1 {
		t.Fatalf("unexpected status codes map: %+v", summary.StatusCodes)
	}
	if summary.Errors[io.ErrUnexpectedEOF.Error()] != 1 {
		t.Fatalf("unexpected errors map: %+v", summary.Errors)
	}
	if summary.SuccessfulLatency.Average != 10*time.Millisecond {
		t.Fatalf("unexpected successful latency: %+v", summary.SuccessfulLatency)
	}
	assertFloatEqual(t, summary.EstimatedThroughput, 80)
	assertFloatEqual(t, summary.SuccessfulThroughput, 20)
}

func TestCalculateStatsSuccessfulLatencyPercentiles(t *testing.T) {
	results := make([]stats.RequestResult, 0, 102)
	for milliseconds := 1; milliseconds <= 100; milliseconds++ {
		results = append(results, stats.RequestResult{
			Duration:   time.Duration(milliseconds) * time.Millisecond,
			StatusCode: 200,
		})
	}
	results = append(results,
		stats.RequestResult{Duration: time.Microsecond, StatusCode: 429},
		stats.RequestResult{Duration: time.Microsecond, Error: io.ErrUnexpectedEOF},
	)

	summary := stats.CalculateStats(results, 2*time.Second)
	latency := summary.SuccessfulLatency

	if summary.AttemptedRequests != 102 || summary.SuccessfulRequests != 100 || summary.FailedRequests != 2 {
		t.Fatalf("unexpected request counts: %+v", summary)
	}
	if latency.Minimum != time.Millisecond || latency.Maximum != 100*time.Millisecond {
		t.Fatalf("unexpected latency range: %+v", latency)
	}
	if latency.Average != 50*time.Millisecond+500*time.Microsecond {
		t.Fatalf("expected 50.5ms average, got %v", latency.Average)
	}
	if latency.P50 != 50*time.Millisecond ||
		latency.P90 != 90*time.Millisecond ||
		latency.P95 != 95*time.Millisecond ||
		latency.P99 != 99*time.Millisecond {
		t.Fatalf("unexpected percentiles: %+v", latency)
	}
	assertFloatEqual(t, summary.EstimatedThroughput, 51)
	assertFloatEqual(t, summary.SuccessfulThroughput, 50)
}

func TestCalculateStatsWithoutSuccessfulRequests(t *testing.T) {
	results := []stats.RequestResult{
		{Duration: time.Millisecond, StatusCode: 429},
		{Duration: 2 * time.Millisecond, Error: io.ErrUnexpectedEOF},
	}

	summary := stats.CalculateStats(results, time.Second)
	if summary.SuccessfulLatency != (stats.LatencyStats{}) {
		t.Fatalf("expected no successful latency samples, got %+v", summary.SuccessfulLatency)
	}
	if summary.SuccessfulThroughput != 0 {
		t.Fatalf("expected zero successful throughput, got %f", summary.SuccessfulThroughput)
	}
}

func assertFloatEqual(t *testing.T, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 0.000001 {
		t.Fatalf("expected %.6f, got %.6f", expected, actual)
	}
}

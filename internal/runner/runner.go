package runner

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/mbrik/CLI-Benchmarking-Tool/internal/config"
	"github.com/mbrik/CLI-Benchmarking-Tool/internal/stats"
)

// RunBenchmark executes the configured requests and returns their aggregate metrics.
func RunBenchmark(ctx context.Context, cfg config.BenchmarkConfig) stats.Summary {
	startTime := time.Now()
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.TotalRequests <= 0 {
		return stats.NewAccumulator(0).Finalize(time.Since(startTime))
	}

	workerCount := min(cfg.Concurrency, cfg.TotalRequests)
	client := NewHTTPClient(workerCount, cfg.Request.Timeout)
	defer client.CloseIdleConnections()

	// Paced jobs use direct handoff so queued work cannot restart in a burst.
	jobBuffer := workerCount
	if cfg.RequestsPerSecond > 0 {
		jobBuffer = 0
	}
	jobs := make(chan struct{}, jobBuffer)
	results := make(chan stats.RequestResult, workerCount)

	var wg sync.WaitGroup
	wg.Add(workerCount)
	// Each worker processes one request at a time and then takes the next job.
	for range workerCount {
		go func() {
			defer wg.Done()
			for range jobs {
				if ctx.Err() != nil {
					return
				}
				results <- SendRequest(ctx, client, &cfg.Request)
			}
		}()
	}

	// Produce jobs independently so workers can begin before every job is queued.
	go produceJobs(ctx, jobs, cfg.TotalRequests, cfg.RequestsPerSecond)

	// Results are complete only after every worker has stopped sending.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate each result immediately instead of retaining every full result.
	accumulator := stats.NewAccumulator(workerCount)
	for result := range results {
		accumulator.Add(result)
	}

	return accumulator.Finalize(time.Since(startTime))
}

func produceJobs(
	ctx context.Context,
	jobs chan<- struct{},
	totalRequests int,
	requestsPerSecond float64,
) {
	defer close(jobs)
	if requestsPerSecond == 0 {
		for range totalRequests {
			select {
			case jobs <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
		return
	}

	intervalNanoseconds := float64(time.Second) / requestsPerSecond
	intervalNanoseconds = max(1, min(intervalNanoseconds, float64(math.MaxInt64)))
	interval := time.Duration(intervalNanoseconds)
	timer := time.NewTimer(0)
	defer timer.Stop()

	for range totalRequests {
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}

		select {
		case jobs <- struct{}{}:
		case <-ctx.Done():
			return
		}
		timer.Reset(interval)
	}
}

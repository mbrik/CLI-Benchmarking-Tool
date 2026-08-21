# Architecture

## Purpose

API Benchmark is a command-line HTTP load generator. A benchmark repeats one
configured HTTP request through a bounded worker pool and reports aggregate
throughput, latency, status code, and error metrics.

The tool uses a closed-loop worker model: each worker starts its next request
after its current request finishes. Concurrency controls the maximum number of
requests running at the same time. Optional pacing limits how quickly requests
may start. Slow responses can make the actual request rate lower than this limit.

## Components

- `main.go` is the application entry point. It creates the process-level
  cancellation context, invokes the CLI command, and handles the final process
  exit.
- `cmd/root.go` defines CLI flags, constructs the benchmark configuration,
  validates it, and coordinates execution and reporting.
- `cmd/report/report.go` renders text or JSON configuration and summary data.
  Sensitive header values are redacted only for display.
- `internal/config/config.go` owns request and benchmark configuration, header
  parsing, and validation.
- `internal/runner/runner.go` owns the worker pool, channel lifecycle,
  cancellation, and streaming result collection.
- `internal/runner/worker.go` owns the HTTP client and execution of one request.
- `internal/stats/stats.go` aggregates request outcomes and calculates final
  metrics.
- `internal/stats/result.go` defines the data exchanged between request
  execution, aggregation, and reporting.

## Execution Flow

```text
CLI flags
    |
    v
BenchmarkConfig validation
    |
    v
Bounded job channel --> worker goroutines --> bounded result channel
                                                 |
                                                 v
                                      streaming stats accumulator
                                                 |
                                                 v
                                               Summary
                                                 |
                                                 v
                                             report output
```

1. `main` creates a context cancelled by an interrupt signal or `SIGTERM`.
2. `cmd.Execute` parses flags and converts repeated header arguments into the
   request header map.
3. `BenchmarkConfig.Validate` rejects invalid request counts, concurrency,
   methods, URLs, headers, and timeouts before workers start.
4. `RunBenchmark` creates one HTTP client and a worker pool sized to the smaller
   of concurrency and total requests.
5. A producer goroutine sends request jobs until the configured total is reached
   or the context is cancelled. When a request rate is configured, it spaces job
   handoffs globally across the worker pool.
6. Each worker processes one job at a time through `SendRequest`, then takes the
   next available job.
7. Completed request results are aggregated as they arrive. Full request results
   are not retained after aggregation.
8. Once all workers exit, the result channel closes and the accumulator produces
   the final summary.
9. The CLI prints the summary as terminal text or one JSON object. An interrupted
   run prints partial metrics and then returns an interruption error.

## Worker Pool

For `N` total requests and concurrency `C`:

- At most `min(N, C)` worker goroutines are created.
- At most `C` HTTP requests can be in flight.
- The job and result channel capacities are bounded by the worker count.
- A worker handles multiple requests sequentially until the job channel closes.
- A positive request rate spaces job handoffs; zero leaves request starts
  unlimited.

For example, `-n 1000 -c 10` queues 1,000 request jobs and uses 10 workers to
process them. Each worker takes another job after its current request finishes.
Adding `-rate 2` limits the complete pool to at most two new request starts per
second.

The producer closes the job channel after producing all jobs. Workers are the
only result senders. A `sync.WaitGroup` tracks worker completion, and a separate
goroutine closes the result channel only after every worker has stopped sending.
The runner drains results while workers are active, preventing a bounded result
channel from deadlocking the pool.

No mutex protects benchmark results because only the runner goroutine mutates the
stats accumulator. Channels transfer each request result to that single owner.

## HTTP Request Lifecycle

Every worker shares one concurrency-scaled `http.Client` and transport. Each
request receives a new body reader and is created with the benchmark context.
Headers are copied from the validated request configuration.

Latency starts immediately before `client.Do` and ends after the response body is
fully copied to `io.Discard` and closed. Reading the body allows connection reuse
and ensures latency includes the complete HTTP response rather than headers only.
A request is failed if sending, reading, or closing the response fails. When a
response was received, its status code is retained even if its body later fails.

Idle pooled connections are closed when the benchmark finishes.

## Metrics

A request is successful when it has no execution error and its final HTTP status
is in the `200-399` range. All other attempted requests are failures.

- **Elapsed time** is wall-clock benchmark execution time.
- **Estimated throughput** is attempted requests divided by elapsed seconds.
- **Successful throughput** is successful requests divided by elapsed seconds.
- **Average latency** is the arithmetic mean of successful request durations.
- **Minimum and maximum latency** are the fastest and slowest successful request.
- **P50, P90, P95, and P99** use the nearest-rank method over sorted successful
  request durations.

Failed request durations are excluded from latency statistics so fast failures,
such as rate-limit responses, do not make successful endpoint latency appear
better than it was. Status codes and transport errors are still counted in their
respective distributions.

## Memory Model

Channel and worker memory scale with concurrency rather than total requests. The
runner aggregates counts, status codes, and errors immediately and does not keep
a complete `RequestResult` slice.

Exact percentiles require retaining each successful request duration until the
run completes. Latency storage is therefore `O(S)`, where `S` is the number of
successful requests, and percentile calculation sorts those durations in place.
Replacing this with fixed memory would require approximate histogram or streaming
quantile semantics.

## Cancellation

Process interrupt signals and `SIGTERM` cancel the benchmark context. Cancellation
stops the producer, interrupts in-flight HTTP requests, and prevents workers from
starting additional requests. Results from requests that started are included in
the partial summary; jobs that never started are not counted as attempted.

`SIGKILL` cannot be intercepted and therefore cannot produce a graceful shutdown
or partial report.

## Current Constraints

- The benchmark repeats one request configuration for the complete run.
- Request pacing is a maximum start rate, not a guaranteed open-loop arrival rate.
- Go's default redirect behavior is used, so metrics describe the final response
  after followed redirects.
- Percentiles are exact and retain successful durations in memory.
- Results are written to standard output as text or JSON. File output is not
  provided.

## Verification

The test suite uses local `httptest` servers to verify request construction,
response-body latency, truncated responses, worker concurrency, result counts,
cancellation, status aggregation, throughput, and latency percentiles. The race
detector verifies the worker and cancellation paths do not introduce shared-memory
races.

```bash
go test ./...
go test -race ./...
```

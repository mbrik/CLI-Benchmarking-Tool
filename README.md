# API Benchmark

API Benchmark is a lightweight command-line tool for measuring HTTP endpoint
throughput, latency, status codes, and request failures. It repeats one request
configuration through a bounded goroutine worker pool and prints the result as
terminal text or JSON.

## Features

- Bounded worker pool with configurable concurrency.
- Optional maximum request start rate for paced benchmarks.
- Support for any valid HTTP method.
- Repeatable custom headers and optional request bodies.
- Per-request timeout covering the complete HTTP exchange.
- Connection pooling and response-body draining for connection reuse.
- Estimated and successful throughput.
- Average, minimum, maximum, P50, P90, P95, and P99 successful latency.
- HTTP status code and request error distributions.
- Text output for humans and JSON output for scripts.
- Sensitive header redaction in displayed configuration.
- Graceful cancellation for interrupt signals and `SIGTERM`.

## Requirements

- Go 1.26.4 or later.

## Installation

Clone and build the executable:

```bash
git clone https://github.com/mbrik/CLI-Benchmarking-Tool.git
cd CLI-Benchmarking-Tool
go build -o api-benchmark .
```

The executable name comes from the `-o` argument. On Windows, use:

```powershell
go build -o api-benchmark.exe .
```

During development, the tool can also run without a separate build step:

```bash
go run . -url "http://localhost:8080/health" -n 100 -c 10
```

## Usage

```text
api-benchmark [flags]

  -url string
        Target URL (default "http://localhost:8080")
  -m, -method string
        HTTP method (default "GET")
  -n int
        Total number of requests (default 1000)
  -c int
        Number of concurrent workers (default 10)
  -r, -rate float
        Maximum requests started per second; 0 means unlimited (default 0)
  -H, -header value
        Custom header in "Name: Value" form; repeat for multiple headers
  -d, -data, -body string
        Request body
  -t, -timeout duration
        Per-request timeout (default 10s)
  -format string
        Output format: text or json (default "text")
```

Total requests, concurrency, timeout, method, URL, and headers are validated
before workers start. Concurrency cannot exceed the total request count.

## Examples

### GET endpoint

Run 2,000 total requests using 20 concurrent workers:

```bash
./api-benchmark \
  -url "http://localhost:8080/api/items?limit=20" \
  -n 2000 \
  -c 20
```

### Authenticated POST endpoint

```bash
./api-benchmark \
  -url "http://localhost:8080/api/items" \
  -method POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_TOKEN" \
  -data '{"name":"Example"}' \
  -n 1000 \
  -c 25 \
  -timeout 15s
```

Authorization, cookie, API key, token, and secret header values are sent to the
target normally but shown as `[REDACTED]` in reports.

### JSON output

JSON mode writes one JSON object to standard output without progress text:

```bash
./api-benchmark \
  -url "http://localhost:8080/api/items" \
  -n 100 \
  -c 10 \
  -format json
```

Example output:

```json
{
  "target": {
    "method": "GET",
    "url": "http://localhost:8080/api/items"
  },
  "configuration": {
    "total_requests": 100,
    "concurrency": 10,
    "max_requests_per_second": 0,
    "timeout_seconds": 10,
    "headers": {},
    "body_size_bytes": 0
  },
  "summary": {
    "elapsed_seconds": 0.235240917,
    "attempted_requests": 100,
    "successful_requests": 100,
    "failed_requests": 0,
    "throughput": {
      "estimated_requests_per_second": 425.1,
      "successful_requests_per_second": 425.1
    },
    "successful_latency_ms": {
      "average": 22.939466,
      "minimum": 4.723625,
      "maximum": 114.392625,
      "p50": 18.412,
      "p90": 43.806,
      "p95": 57.114,
      "p99": 92.731
    },
    "status_codes": {
      "200": 100
    },
    "errors": {}
  }
}
```

### Paced requests

Limit the benchmark to at most two new requests per second while allowing up to
five slow requests to overlap:

```bash
./api-benchmark \
  -url "http://localhost:8080/api/analytics/dashboard?period=all&currency=NGN" \
  -n 50 \
  -c 5 \
  -rate 2
```

Here, `-rate 2` starts no more than two requests per second, so request starts
are spaced about 500 milliseconds apart. `-c 5` allows up to five requests to be
running at the same time. If a request takes longer than 500 milliseconds, the
next request may start while the earlier one is still running. If all five
workers are busy, the next request waits, so the actual start rate becomes lower
than two requests per second.

Pacing does not bypass a target's rate limiter. It helps keep a benchmark below
a known limit so successful endpoint behavior can be measured without producing
mostly `429 Too Many Requests` responses.

## Text Output

```text
Benchmark Target: [GET] http://localhost:8080/api/items
Requests: 100 | Concurrency: 10 | Rate: unlimited | Timeout: 10s
Running benchmark, please wait...

==================================
BENCHMARK SUMMARY
==================================
Elapsed Time          : 235.240917ms
Requests Attempted    : 100
Successful Requests   : 100
Failed Requests       : 0
Estimated Throughput  : 425.10 req/sec
Successful Throughput : 425.10 req/sec
----------------------------------
SUCCESSFUL REQUEST LATENCY
P50                    : 18.412ms
P90                    : 43.806ms
P95                    : 57.114ms
P99                    : 92.731ms
Average                : 22.939466ms
Minimum                : 4.723625ms
Maximum                : 114.392625ms
----------------------------------
STATUS CODE DISTRIBUTION
   [200 OK]: 100 responses
==================================
```

When requests fail before receiving an HTTP response, text output includes an
error breakdown:

```text
==================================
BENCHMARK SUMMARY
==================================
Elapsed Time          : 3.142625ms
Requests Attempted    : 10
Successful Requests   : 0
Failed Requests       : 10
Estimated Throughput  : 3182.06 req/sec
Successful Throughput : 0.00 req/sec
----------------------------------
SUCCESSFUL REQUEST LATENCY
No successful requests recorded.
----------------------------------
ERROR BREAKDOWN
   - Get "http://localhost:8080/api/items": dial tcp 127.0.0.1:8080: connect: connection refused: 10 occurrences
==================================
```

JSON output places the same counts in `summary.errors`. HTTP failures such as
`429 Too Many Requests` appear in `summary.status_codes` because the server
returned a response.

## Metric Semantics

- **Elapsed time** is wall-clock benchmark execution time.
- **Attempted requests** are requests that reached a worker and produced a result.
- **Successful requests** have no execution error and a final status from 200 to
  399.
- **Failed requests** include non-success statuses, timeouts, connection errors,
  truncated bodies, and cancellation errors.
- **Estimated throughput** is attempted requests divided by elapsed seconds.
- **Successful throughput** is successful requests divided by elapsed seconds.
- **Request rate** limits how quickly requests may start. Slow responses can make
  the actual request rate lower than this limit.
- **Latency** starts before the HTTP exchange and ends after the complete response
  body has been read and closed.
- **Latency statistics** use successful requests only. Percentiles use the exact
  nearest-rank method.

A `429 Too Many Requests` response is therefore a failed request. It contributes
to estimated throughput and status distribution, but not successful throughput or
successful latency percentiles. The benchmark reports the target's rate limiting;
it does not bypass it.

## Concurrency And Memory

With `N` requests and concurrency `C`, the runner creates at most `min(N, C)`
workers. Each worker handles requests sequentially, so no more than `C` requests
are in flight.

Job and result channel memory scales with concurrency. Request counts, status
codes, and errors are aggregated as results arrive instead of retaining every
full result. Exact percentiles still require one stored duration per successful
request.

The worker model is closed-loop: a worker starts its next request after its
current request finishes. Optional rate pacing also limits how quickly jobs are
handed to ready workers.

See [ARCHITECTURE.md](ARCHITECTURE.md) for the complete execution flow and
concurrency lifecycle.

## Cancellation

Interrupt signals and `SIGTERM` stop job production and cancel in-flight HTTP
requests. The tool prints a partial report containing requests that actually
started, then exits with an interruption error. `SIGKILL` cannot be handled by an
application.

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
```

Tests use local `httptest` servers and do not require an external API.

## Project Structure

```text
.
|-- cmd/
|   |-- report/
|   |   |-- report.go
|   |   `-- report_test.go
|   `-- root.go
|-- internal/
|   |-- config/
|   |   |-- config.go
|   |   `-- config_test.go
|   |-- runner/
|   |   |-- runner.go
|   |   |-- runner_test.go
|   |   `-- worker.go
|   `-- stats/
|       |-- result.go
|       |-- stats.go
|       `-- stats_test.go
|-- ARCHITECTURE.md
|-- README.md
|-- go.mod
`-- main.go
```

Run load tests against disposable test environments or disposable data whenever
possible. Requests to endpoints that create, update, or delete data can produce
real side effects.

# ⚡ CLI Benchmarking Tool (`go-bench`)

A fast, lightweight, and memory-efficient HTTP load generator and benchmarking tool written in Go. Evaluates throughput, latency percentiles, status codes, and network failures with bounded memory and graceful shutdown.

---

## ✨ Features

- **🚀 Bounded Concurrency**: Efficient worker pool architecture with bounded channel capacity and zero memory bloat (`min(N, C)` goroutines).
- **⏱️ Request Rate Pacing**: Optional `-r` / `-rate` flag to space request starts and avoid triggering rate limiters.
- **🔄 Any HTTP Method**: Full support for `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD`, `OPTIONS`, and more.
- **📋 Custom Headers & Body**: Repeatable `-H` / `-header` flags and raw payload support (`-d`, `-data`, `-body`).
- **🔒 Sensitive Header Redaction**: Automatically hides `Authorization`, `Cookie`, tokens, secrets, and API keys in reports.
- **🛡️ Graceful Shutdown**: Listens for `Ctrl+C` (`SIGINT` / `SIGTERM`) to cancel in-flight requests and show partial summaries.
- **📊 Rich Performance Metrics**:
  - **Throughput**: Estimated throughput vs. Successful throughput.
  - **Latency Percentiles**: P50, P90, P95, P99, Average, Min, and Max (calculated strictly on successful `2xx-3xx` requests).
  - **Status Code Distribution**: Full breakdown of all returned HTTP statuses (`200 OK`, `429 Too Many Requests`, etc.).
  - **Error Breakdown**: Network timeouts, connection drops, and truncated response bodies.
- **🤖 Output Formats**: Human-friendly terminal text (default) or machine-readable JSON (`-format json`).

---

## 📦 Installation & Setup

### Prerequisites
- [Go](https://go.dev/dl/) `1.20+` installed.

### Build from Source
```bash
# Clone the repository
git clone https://github.com/mbrik/CLI-Benchmarking-Tool.git
cd CLI-Benchmarking-Tool/go-bench

# Build executable
go build -o go-bench .
```

On Windows, use:
```powershell
go build -o go-bench.exe .
```

During development, the tool can also run directly:
```bash
go run . -url "http://localhost:8080/health" -n 100 -c 10
```

---

## 🛠️ CLI Flags & Usage

```text
go-bench [flags]

Flags:
  -url string
        Target URL to benchmark (default "http://localhost:8080")
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
        Request payload / data string
  -t, -timeout duration
        Per-request timeout duration, e.g. 5s, 10s, 1m (default 10s)
  -format string
        Output format: text or json (default "text")
  -help
        Show help documentation
```

> Total requests, concurrency, timeout, method, URL, and headers are validated before workers start. Concurrency cannot exceed the total request count.

---

## 🚀 Examples

### 1. Basic GET Benchmark
Run 2,000 total requests using 20 concurrent workers:
```bash
./go-bench -url "http://localhost:8080/api/items?limit=20" -n 2000 -c 20
```

### 2. Authenticated POST Endpoint
Send 1,000 POST requests with JSON payload and auth headers:
```bash
./go-bench \
  -url "http://localhost:8080/api/items" \
  -method POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my_secret_token" \
  -data '{"name":"Example"}' \
  -n 1000 \
  -c 25 \
  -timeout 15s
```

*Note: Authorization, cookie, API key, token, and secret header values are sent over the wire normally but displayed as `[REDACTED]` in reports.*

### 3. Paced Requests
Limit the benchmark to at most 2 new requests per second while allowing up to 5 overlapping requests:
```bash
./go-bench \
  -url "http://localhost:8080/api/analytics/dashboard" \
  -n 50 \
  -c 5 \
  -rate 2
```

### 4. Machine-Readable JSON Output
Pipe benchmark statistics directly into `jq` or CI/CD pipelines:
```bash
./go-bench -url "http://localhost:8080/api/items" -n 100 -c 10 -format json
```

Example JSON output:
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

---

## 📊 Sample Output

### Text Mode (Default)
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

When errors occur:
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

---

## 📐 Metric Semantics

- **Elapsed time**: Wall-clock benchmark execution time.
- **Attempted requests**: Requests that reached a worker and produced a result.
- **Successful requests**: Requests with no execution error and HTTP status `200-399`.
- **Failed requests**: Non-success statuses (`4xx`, `5xx`), timeouts, connection drops, truncated response bodies, and cancellations.
- **Estimated throughput**: Total attempted requests divided by elapsed seconds.
- **Successful throughput**: Successful requests divided by elapsed seconds.
- **Latency statistics**: Calculated strictly on successful requests using exact nearest-rank percentiles.

---

## 🧪 Running Tests

```bash
go test ./...
go test -race ./...
go vet ./...
```

---

## 📁 Project Structure

```text
go-bench/
├── cmd/
│   ├── report/
│   │   ├── report.go        # Terminal & JSON presentation formatting
│   │   └── report_test.go   # Redaction and formatting unit tests
│   └── root.go              # CLI flags & coordination
├── internal/
│   ├── config/
│   │   ├── config.go        # Configuration models & RFC-compliant validation
│   │   └── config_test.go   # Configuration validation test suite
│   ├── runner/
│   │   ├── runner.go        # Worker pool, rate-pacer, & cancellation lifecycle
│   │   ├── runner_test.go   # Concurrency, pacing, & cancellation tests
│   │   └── worker.go        # HTTP client, connection pooling, & request executor
│   └── stats/
│       ├── result.go        # Result structs & latency definitions
│       ├── stats.go         # Streaming stats accumulator & percentiles
│       └── stats_test.go    # Latency percentiles & stats tests
├── ARCHITECTURE.md          # Architectural deep-dive & design documentation
├── README.md                # Project documentation
├── go.mod                   # Go module definition
└── main.go                  # Signal context & entry point
```

---

## 📄 License
This project is open source and available under the [MIT License](LICENSE).

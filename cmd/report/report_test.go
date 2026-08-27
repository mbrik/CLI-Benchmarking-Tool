package report

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mbrik/CLI-Benchmarking-Tool/internal/config"
	"github.com/mbrik/CLI-Benchmarking-Tool/internal/stats"
)

func TestDisplayHeaderValueRedactsCredentials(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "Authorization", value: "Bearer private", want: "[REDACTED]"},
		{name: "Cookie", value: "session=private", want: "[REDACTED]"},
		{name: "X-API-Key", value: "private", want: "[REDACTED]"},
		{name: "X-Access-Token", value: "private", want: "[REDACTED]"},
		{name: "Content-Type", value: "application/json", want: "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := displayHeaderValue(tt.name, tt.value); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestWriteJSONReport(t *testing.T) {
	cfg := config.BenchmarkConfig{
		Request: config.RequestConfig{
			Method: http.MethodPost,
			URL:    "https://example.com/api/items",
			Headers: map[string]string{
				"Authorization": "Bearer private-token",
				"Content-Type":  "application/json",
			},
			Body:    []byte(`{"name":"item"}`),
			Timeout: 1500 * time.Millisecond,
		},
		TotalRequests:     100,
		Concurrency:       10,
		RequestsPerSecond: 2.5,
	}
	summary := stats.Summary{
		AttemptedRequests:    100,
		SuccessfulRequests:   90,
		FailedRequests:       10,
		ElapsedTime:          2 * time.Second,
		EstimatedThroughput:  50,
		SuccessfulThroughput: 45,
		SuccessfulLatency: stats.LatencyStats{
			Average: 25 * time.Millisecond,
			Minimum: 10 * time.Millisecond,
			Maximum: 80 * time.Millisecond,
			P50:     20 * time.Millisecond,
			P90:     40 * time.Millisecond,
			P95:     50 * time.Millisecond,
			P99:     70 * time.Millisecond,
		},
		StatusCodes: map[int]int{200: 90, 429: 10},
		Errors:      map[string]int{},
	}

	var output bytes.Buffer
	if err := WriteJSON(&output, cfg, summary); err != nil {
		t.Fatalf("expected JSON report, got %v", err)
	}
	if strings.Contains(output.String(), "private-token") {
		t.Fatal("expected authorization token to be redacted")
	}

	var report jsonReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatalf("expected valid JSON, got %v", err)
	}
	if report.Target.Method != http.MethodPost || report.Target.URL != cfg.Request.URL {
		t.Fatalf("unexpected target: %+v", report.Target)
	}
	if report.Configuration.TimeoutSeconds != 1.5 {
		t.Fatalf("expected 1.5 timeout seconds, got %f", report.Configuration.TimeoutSeconds)
	}
	if report.Configuration.MaxRequestsPerSecond != 2.5 {
		t.Fatalf("expected 2.5 requests per second, got %f", report.Configuration.MaxRequestsPerSecond)
	}
	if report.Configuration.Headers["Authorization"] != "[REDACTED]" {
		t.Fatalf("expected redacted headers, got %+v", report.Configuration.Headers)
	}
	if report.Summary.SuccessfulLatencyMS.P99 != 70 {
		t.Fatalf("expected 70ms P99, got %f", report.Summary.SuccessfulLatencyMS.P99)
	}
	if report.Summary.Throughput.SuccessfulRequestsPerSecond != 45 {
		t.Fatalf("expected 45 successful requests per second, got %f", report.Summary.Throughput.SuccessfulRequestsPerSecond)
	}
}

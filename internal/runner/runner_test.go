package runner_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mbrik/CLI-Benchmarking-Tool/internal/config"
	"github.com/mbrik/CLI-Benchmarking-Tool/internal/runner"
	"github.com/mbrik/CLI-Benchmarking-Tool/internal/stats"
)

func TestSendRequestCustomMethodAndHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.Header.Get("X-Custom-Header") != "TestValue" {
			t.Errorf("expected X-Custom-Header to be TestValue, got %s", r.Header.Get("X-Custom-Header"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"hello":"world"}` {
			t.Errorf("unexpected body: %s, err: %v", string(body), err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	client := runner.NewHTTPClient(5, 5*time.Second)
	reqCfg := config.RequestConfig{
		Method: "POST",
		URL:    server.URL,
		Headers: map[string]string{
			"X-Custom-Header": "TestValue",
		},
		Body:    []byte(`{"hello":"world"}`),
		Timeout: 5 * time.Second,
	}

	result := runner.SendRequest(context.Background(), client, &reqCfg)
	if result.Error != nil {
		t.Fatalf("expected no error, got %v", result.Error)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", result.StatusCode)
	}
}

func TestSendRequestIncludesResponseBodyReadInLatency(t *testing.T) {
	const bodyDelay = 50 * time.Millisecond

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(bodyDelay)
		_, _ = w.Write([]byte("complete"))
	}))
	defer server.Close()

	client := runner.NewHTTPClient(1, time.Second)
	defer client.CloseIdleConnections()
	requestConfig := config.RequestConfig{
		Method:  http.MethodGet,
		URL:     server.URL,
		Timeout: time.Second,
	}

	result := runner.SendRequest(context.Background(), client, &requestConfig)
	if result.Error != nil {
		t.Fatalf("expected no error, got %v", result.Error)
	}
	if result.Duration < bodyDelay {
		t.Fatalf("expected latency to include %v body delay, got %v", bodyDelay, result.Duration)
	}
}

func TestSendRequestReportsTruncatedResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("incomplete"))
	}))
	defer server.Close()

	client := runner.NewHTTPClient(1, time.Second)
	defer client.CloseIdleConnections()
	requestConfig := config.RequestConfig{
		Method:  http.MethodGet,
		URL:     server.URL,
		Timeout: time.Second,
	}

	result := runner.SendRequest(context.Background(), client, &requestConfig)
	if !errors.Is(result.Error, io.ErrUnexpectedEOF) {
		t.Fatalf("expected unexpected EOF, got %v", result.Error)
	}
	if result.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 to be retained, got %d", result.StatusCode)
	}
}

func TestRunBenchmark(t *testing.T) {
	var requestCount atomic.Int64
	var activeRequests atomic.Int64
	var maximumActiveRequests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		active := activeRequests.Add(1)
		defer activeRequests.Add(-1)

		for {
			maximum := maximumActiveRequests.Load()
			if active <= maximum || maximumActiveRequests.CompareAndSwap(maximum, active) {
				break
			}
		}

		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	benchCfg := config.BenchmarkConfig{
		Request: config.RequestConfig{
			Method:  "GET",
			URL:     server.URL,
			Timeout: 5 * time.Second,
		},
		TotalRequests: 105,
		Concurrency:   10,
	}

	summary := runner.RunBenchmark(context.Background(), benchCfg)
	if requestCount.Load() != 105 {
		t.Fatalf("expected server to receive 105 requests, got %d", requestCount.Load())
	}
	if maximumActiveRequests.Load() > int64(benchCfg.Concurrency) {
		t.Fatalf(
			"expected no more than %d concurrent requests, got %d",
			benchCfg.Concurrency,
			maximumActiveRequests.Load(),
		)
	}

	if summary.AttemptedRequests != 105 {
		t.Fatalf("expected 105 attempted requests, got %d", summary.AttemptedRequests)
	}
	if summary.SuccessfulRequests != 105 {
		t.Fatalf("expected 105 successful requests, got %d", summary.SuccessfulRequests)
	}
	if summary.FailedRequests != 0 {
		t.Fatalf("expected 0 failed requests, got %d", summary.FailedRequests)
	}
	if summary.StatusCodes[http.StatusOK] != 105 {
		t.Fatalf("expected 105 HTTP 200 responses, got %d", summary.StatusCodes[http.StatusOK])
	}
	if summary.ElapsedTime <= 0 {
		t.Fatalf("expected positive elapsed time, got %v", summary.ElapsedTime)
	}
	if summary.SuccessfulLatency.Average <= 0 {
		t.Fatalf("expected positive average latency, got %v", summary.SuccessfulLatency.Average)
	}
	if summary.EstimatedThroughput <= 0 || summary.SuccessfulThroughput <= 0 {
		t.Fatalf("expected positive throughput metrics, got %+v", summary)
	}
}

func TestRunBenchmarkCancellation(t *testing.T) {
	const concurrency = 4
	started := make(chan struct{}, concurrency)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-r.Context().Done()
	}))
	defer server.Close()

	benchCfg := config.BenchmarkConfig{
		Request: config.RequestConfig{
			Method:  http.MethodGet,
			URL:     server.URL,
			Timeout: 5 * time.Second,
		},
		TotalRequests: 100,
		Concurrency:   concurrency,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan stats.Summary, 1)
	go func() {
		done <- runner.RunBenchmark(ctx, benchCfg)
	}()

	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for a request to start")
	}

	var summary stats.Summary
	select {
	case summary = <-done:
	case <-time.After(time.Second):
		t.Fatal("benchmark did not stop after cancellation")
	}

	if summary.AttemptedRequests == 0 || summary.AttemptedRequests > concurrency {
		t.Fatalf("expected 1-%d attempted requests, got %d", concurrency, summary.AttemptedRequests)
	}
	if summary.SuccessfulRequests != 0 || summary.FailedRequests != summary.AttemptedRequests {
		t.Fatalf("expected every attempted request to fail after cancellation, got %+v", summary)
	}
	if len(summary.Errors) == 0 {
		t.Fatal("expected cancellation errors to be recorded")
	}
}

func TestRunBenchmarkRequestRate(t *testing.T) {
	const (
		totalRequests     = 4
		requestsPerSecond = 20
		minimumSpacing    = 40 * time.Millisecond
	)

	var mu sync.Mutex
	requestTimes := make([]time.Time, 0, totalRequests)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestTimes = append(requestTimes, time.Now())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	benchCfg := config.BenchmarkConfig{
		Request: config.RequestConfig{
			Method:  http.MethodGet,
			URL:     server.URL,
			Timeout: time.Second,
		},
		TotalRequests:     totalRequests,
		Concurrency:       totalRequests,
		RequestsPerSecond: requestsPerSecond,
	}

	summary := runner.RunBenchmark(context.Background(), benchCfg)
	if summary.SuccessfulRequests != totalRequests {
		t.Fatalf("expected %d successful requests, got %d", totalRequests, summary.SuccessfulRequests)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(requestTimes) != totalRequests {
		t.Fatalf("expected %d recorded request times, got %d", totalRequests, len(requestTimes))
	}
	for i := 1; i < len(requestTimes); i++ {
		spacing := requestTimes[i].Sub(requestTimes[i-1])
		if spacing < minimumSpacing {
			t.Fatalf("expected at least %v between requests, got %v", minimumSpacing, spacing)
		}
	}
}

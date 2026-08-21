package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mbrik/CLI-Benchmarking-Tool/cmd/report"
	"github.com/mbrik/CLI-Benchmarking-Tool/internal/config"
	"github.com/mbrik/CLI-Benchmarking-Tool/internal/runner"
)

// headerFlags is a custom flag type to allow multiple -H / -header arguments
type headerFlags []string

func (h *headerFlags) String() string {
	return strings.Join(*h, ", ")
}

func (h *headerFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

// Execute parses CLI flags and runs the benchmark process.
func Execute(ctx context.Context) error {
	// Define command-line flags
	urlFlag := flag.String("url", "http://localhost:8080", "Target URL to benchmark")
	methodFlag := flag.String("m", "GET", "HTTP method (GET, POST, PUT, DELETE, PATCH, etc.)")
	flag.StringVar(methodFlag, "method", "GET", "HTTP method (alias for -m)")

	requestsFlag := flag.Int("n", 1000, "Total number of requests to perform")
	concurrencyFlag := flag.Int("c", 10, "Number of concurrent workers")
	rateFlag := flag.Float64("r", 0, "Maximum requests started per second; 0 means unlimited")
	flag.Float64Var(rateFlag, "rate", 0, "Maximum requests started per second (alias for -r)")
	formatFlag := flag.String("format", "text", "Output format: text or json")

	dataFlag := flag.String("d", "", "HTTP request body / data")
	flag.StringVar(dataFlag, "data", "", "HTTP request body / data (alias for -d)")
	flag.StringVar(dataFlag, "body", "", "HTTP request body / data (alias for -d)")

	var headers headerFlags
	flag.Var(&headers, "H", "Custom HTTP header (e.g. -H \"Content-Type: application/json\"), repeatable")
	flag.Var(&headers, "header", "Custom HTTP header (alias for -H), repeatable")

	timeoutFlag := flag.Duration("t", 10*time.Second, "Per-request timeout (e.g. 5s, 10s)")
	flag.DurationVar(timeoutFlag, "timeout", 10*time.Second, "Per-request timeout (alias for -t)")

	flag.Parse()

	method := strings.ToUpper(strings.TrimSpace(*methodFlag))
	outputFormat := strings.ToLower(strings.TrimSpace(*formatFlag))
	if outputFormat != "text" && outputFormat != "json" {
		return fmt.Errorf("invalid output format %q: expected text or json", *formatFlag)
	}

	headerMap, err := config.ParseHeaders([]string(headers))
	if err != nil {
		return err
	}

	reqConfig := config.RequestConfig{
		Method:  method,
		URL:     *urlFlag,
		Headers: headerMap,
		Body:    []byte(*dataFlag),
		Timeout: *timeoutFlag,
	}

	benchConfig := config.BenchmarkConfig{
		Request:           reqConfig,
		TotalRequests:     *requestsFlag,
		Concurrency:       *concurrencyFlag,
		RequestsPerSecond: *rateFlag,
	}
	if err := benchConfig.Validate(); err != nil {
		return fmt.Errorf("invalid benchmark configuration: %w", err)
	}

	if outputFormat == "text" {
		report.PrintBenchmarkConfiguration(benchConfig)
	}

	summary := runner.RunBenchmark(ctx, benchConfig)
	if outputFormat == "json" {
		if err := report.WriteJSON(os.Stdout, benchConfig, summary); err != nil {
			return fmt.Errorf("write JSON report: %w", err)
		}
	} else {
		report.PrintSummary(summary)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("benchmark interrupted: %w", err)
	}

	return nil
}

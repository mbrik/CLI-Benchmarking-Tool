package config

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RequestConfig contains the settings for one HTTP request.
type RequestConfig struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
	Timeout time.Duration
}

// BenchmarkConfig contains the settings for a complete benchmark run.
type BenchmarkConfig struct {
	Request           RequestConfig
	TotalRequests     int
	Concurrency       int
	RequestsPerSecond float64
}

// ParseHeaders converts repeated CLI header values into request headers.
func ParseHeaders(values []string) (map[string]string, error) {
	headers := make(map[string]string, len(values))
	for _, value := range values {
		name, headerValue, found := strings.Cut(value, ":")
		if !found {
			return nil, fmt.Errorf("invalid header %q: expected Name: Value", value)
		}

		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("invalid header %q: name cannot be empty", value)
		}
		headers[name] = strings.TrimSpace(headerValue)
	}

	return headers, nil
}

// Validate checks configuration that must be valid before workers start.
func (c BenchmarkConfig) Validate() error {
	if c.TotalRequests <= 0 {
		return fmt.Errorf("total requests must be greater than zero")
	}
	if c.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be greater than zero")
	}
	if c.Concurrency > c.TotalRequests {
		return fmt.Errorf("concurrency cannot exceed total requests")
	}
	if c.RequestsPerSecond < 0 || math.IsNaN(c.RequestsPerSecond) || math.IsInf(c.RequestsPerSecond, 0) {
		return fmt.Errorf("request rate must be zero or a positive finite number")
	}
	if err := c.Request.Validate(); err != nil {
		return fmt.Errorf("request configuration: %w", err)
	}

	return nil
}

// Validate checks request settings before any network work begins.
func (c RequestConfig) Validate() error {
	method := strings.TrimSpace(c.Method)
	if method == "" {
		return fmt.Errorf("HTTP method cannot be empty")
	}

	parsedURL, err := url.Parse(strings.TrimSpace(c.URL))
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must include a host")
	}
	if _, err := http.NewRequest(method, parsedURL.String(), nil); err != nil {
		return fmt.Errorf("invalid HTTP method or URL: %w", err)
	}
	for key, value := range c.Headers {
		if !isValidHeaderName(key) {
			return fmt.Errorf("invalid HTTP headers: invalid name %q", key)
		}
		if !isValidHeaderValue(value) {
			return fmt.Errorf("invalid HTTP headers: invalid value for %q", key)
		}
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}

	return nil
}

func isValidHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := range len(name) {
		if !isHeaderTokenByte(name[i]) {
			return false
		}
	}

	return true
}

func isHeaderTokenByte(value byte) bool {
	if value >= '0' && value <= '9' ||
		value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' {
		return true
	}

	switch value {
	case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
		return true
	default:
		return false
	}
}

func isValidHeaderValue(value string) bool {
	for i := range len(value) {
		character := value[i]
		if character < ' ' && character != '\t' || character == 0x7f {
			return false
		}
	}

	return true
}

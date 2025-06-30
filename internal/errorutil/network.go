package errorutil

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// NetworkError provides structured error information for network operations
type NetworkError struct {
	Operation  string
	URL        string
	StatusCode int
	Err        error
	Retryable  bool
}

func (e *NetworkError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s failed for %s (HTTP %d): %v", e.Operation, e.URL, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("%s failed for %s: %v", e.Operation, e.URL, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// IsRetryable returns whether this error should be retried
func (e *NetworkError) IsRetryable() bool {
	return e.Retryable
}

// ClassifyHTTPError categorizes HTTP errors for consistent handling
// Consolidates the pattern of HTTP response checking and error classification
func ClassifyHTTPError(resp *http.Response, err error, operation, url string) *NetworkError {
	netErr := &NetworkError{
		Operation: operation,
		URL:       url,
		Err:       err,
	}

	if resp != nil {
		netErr.StatusCode = resp.StatusCode
		netErr.Retryable = isRetryableStatusCode(resp.StatusCode)
		
		if err == nil {
			// Create error from status code if none provided
			netErr.Err = fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
		}
	} else if err != nil {
		// Network-level error (connection issues, timeouts, etc.)
		netErr.Retryable = isRetryableNetworkError(err)
	}

	return netErr
}

// isRetryableStatusCode determines if an HTTP status code indicates a retryable error
func isRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case http.StatusTooManyRequests,      // 429
		 http.StatusBadGateway,           // 502
		 http.StatusServiceUnavailable,   // 503
		 http.StatusGatewayTimeout:       // 504
		return true
	case http.StatusInternalServerError:  // 500 - sometimes retryable
		return true
	default:
		return false
	}
}

// isRetryableNetworkError determines if a network error is retryable
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"network unreachable",
		"no route to host",
		"connection timed out",
		"i/o timeout",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// DefaultRetryConfig returns sensible defaults for HTTP retries
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// CalculateBackoff calculates exponential backoff delay for retry attempts
func (c RetryConfig) CalculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return c.BaseDelay
	}

	delay := c.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * c.Multiplier)
		if delay > c.MaxDelay {
			return c.MaxDelay
		}
	}
	return delay
}

// ShouldRetry determines if an error should be retried given the attempt count
func (c RetryConfig) ShouldRetry(err error, attempt int) bool {
	if attempt >= c.MaxRetries {
		return false
	}

	if netErr, ok := err.(*NetworkError); ok {
		return netErr.IsRetryable()
	}

	// For non-NetworkError types, check if it looks like a retryable network error
	return isRetryableNetworkError(err)
}

// ValidateHTTPResponse checks HTTP response for common error conditions
// Consolidates the pattern of response validation found across the codebase
func ValidateHTTPResponse(resp *http.Response, operation, url string) error {
	if resp == nil {
		return &NetworkError{
			Operation: operation,
			URL:       url,
			Err:       fmt.Errorf("nil response"),
			Retryable: false,
		}
	}

	// Success status codes (2xx)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Create appropriate error based on status code
	var errMsg string
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		errMsg = "authentication failed"
	case http.StatusForbidden:
		errMsg = "access denied"
	case http.StatusNotFound:
		errMsg = "resource not found"
	case http.StatusTooManyRequests:
		errMsg = "rate limit exceeded"
	case http.StatusInternalServerError:
		errMsg = "server error"
	case http.StatusBadGateway:
		errMsg = "bad gateway"
	case http.StatusServiceUnavailable:
		errMsg = "service unavailable"
	case http.StatusGatewayTimeout:
		errMsg = "gateway timeout"
	default:
		errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	return &NetworkError{
		Operation:  operation,
		URL:        url,
		StatusCode: resp.StatusCode,
		Err:        fmt.Errorf("%s", errMsg),
		Retryable:  isRetryableStatusCode(resp.StatusCode),
	}
}

// ExtractRetryAfter attempts to extract Retry-After header value
// Returns 0 if header not present or invalid
func ExtractRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}

	retryAfter := resp.Header.Get("Retry-After")
	if retryAfter == "" {
		return 0
	}

	// Try parsing as seconds (integer)
	if seconds, err := time.ParseDuration(retryAfter + "s"); err == nil {
		return seconds
	}

	// Could also parse as HTTP date, but for now just return 0
	return 0
}
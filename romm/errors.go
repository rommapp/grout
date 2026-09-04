package romm

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var (
	ErrInvalidHostname   = errors.New("invalid hostname")
	ErrConnectionRefused = errors.New("connection refused")
	ErrTimeout           = errors.New("connection timeout")
	ErrUnauthorized      = errors.New("invalid credentials")
	ErrForbidden         = errors.New("access forbidden")
	ErrServerError       = errors.New("server error")
	ErrConflict          = errors.New("conflict")
	ErrNotFound          = errors.New("not found")
	ErrRateLimited       = errors.New("rate limited")
)

// APIError is a non-2xx response from RomM. It unwraps to the sentinel for its
// status, so callers can ask errors.Is what went wrong rather than reading the
// message.
type APIError struct {
	StatusCode int
	Body       string
	sentinel   error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: status %d, body: %s", e.StatusCode, e.Body)
}

func (e *APIError) Unwrap() error { return e.sentinel }

// statusError describes a non-2xx response.
func statusError(statusCode int, body []byte) error {
	return &APIError{StatusCode: statusCode, Body: string(body), sentinel: sentinelFor(statusCode)}
}

// sentinelFor is the error a status maps to, or nil when it has no more
// specific meaning than "the request failed".
func sentinelFor(statusCode int) error {
	switch {
	case statusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case statusCode == http.StatusForbidden:
		return ErrForbidden
	case statusCode == http.StatusNotFound:
		return ErrNotFound
	case statusCode == http.StatusConflict:
		return ErrConflict
	case statusCode == http.StatusTooManyRequests:
		return ErrRateLimited
	case statusCode >= 500:
		return ErrServerError
	}
	return nil
}

// ConflictError represents a 409 Conflict response from the server,
// typically when uploading a save that conflicts with the current state.
type ConflictError struct {
	ErrorType       string    `json:"error"`
	Message         string    `json:"message"`
	SaveID          int       `json:"save_id"`
	CurrentSaveTime time.Time `json:"current_save_time"`
	DeviceSyncTime  time.Time `json:"device_sync_time"`
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %s (save_id=%d)", e.Message, e.SaveID)
}

func (e *ConflictError) Unwrap() error {
	return ErrConflict
}

func ClassifyError(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		var dnsErr *net.DNSError
		if errors.As(urlErr.Err, &dnsErr) {
			return fmt.Errorf("%w: %s", ErrInvalidHostname, dnsErr.Name)
		}

		var opErr *net.OpError
		if errors.As(urlErr.Err, &opErr) {
			if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
				return fmt.Errorf("%w: host not reachable", ErrConnectionRefused)
			}
			if opErr.Timeout() {
				return fmt.Errorf("%w: host did not respond", ErrTimeout)
			}
		}

	}

	if strings.Contains(errMsg, "context deadline exceeded") ||
		strings.Contains(errMsg, "Client.Timeout exceeded") ||
		strings.Contains(errMsg, "timeout") {
		return fmt.Errorf("%w: host did not respond", ErrTimeout)
	}

	slog.Default().Debug("ClassifyError: unclassified error", "error", err, "error_type", fmt.Sprintf("%T", err), "error_string", errMsg)

	return err
}

package romm

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
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
)

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

type AuthError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication error (status %d): %s", e.StatusCode, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Err
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

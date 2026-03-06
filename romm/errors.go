package romm

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

var (
	ErrInvalidHostname   = errors.New("invalid hostname")
	ErrConnectionRefused = errors.New("connection refused")
	ErrTimeout           = errors.New("connection timeout")
	ErrWrongProtocol     = errors.New("wrong protocol")
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

type ProtocolError struct {
	RequestedProtocol string
	CorrectProtocol   string
	Err               error
}

func (e *ProtocolError) Error() string {
	return fmt.Sprintf("protocol mismatch: use %s instead of %s", e.CorrectProtocol, e.RequestedProtocol)
}

func (e *ProtocolError) Unwrap() error {
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

		innerErrMsg := urlErr.Err.Error()
		if strings.Contains(innerErrMsg, "HTTP response to HTTPS client") ||
			strings.Contains(innerErrMsg, "first record does not look like a TLS handshake") ||
			strings.Contains(innerErrMsg, "malformed HTTP response") ||
			strings.Contains(innerErrMsg, "TLS handshake") {
			return fmt.Errorf("%w: try switching between http and https", ErrWrongProtocol)
		}
	}

	if strings.Contains(errMsg, "HTTP response to HTTPS client") ||
		strings.Contains(errMsg, "first record does not look like a TLS handshake") ||
		strings.Contains(errMsg, "malformed HTTP response") ||
		strings.Contains(errMsg, "TLS handshake") ||
		strings.Contains(errMsg, "http: server gave HTTP response to HTTPS client") {
		return fmt.Errorf("%w: try switching between http and https", ErrWrongProtocol)
	}

	if strings.Contains(errMsg, "context deadline exceeded") ||
		strings.Contains(errMsg, "Client.Timeout exceeded") ||
		strings.Contains(errMsg, "timeout") {
		return fmt.Errorf("%w: host did not respond", ErrTimeout)
	}

	gabagool.GetLogger().Debug("ClassifyError: unclassified error", "error", err, "error_type", fmt.Sprintf("%T", err), "error_string", errMsg)

	return err
}

package auth

import (
	"errors"
	"log/slog"

	"grout/romm"
)

// Failure is why connecting or logging in did not work. The screen turns it
// into something to read; nothing here knows what that says.
type Failure int

const (
	// FailureUnknown is an error grout has nothing specific to say about. It
	// is logged so the cause can be found later.
	FailureUnknown Failure = iota
	FailureHostname
	FailureConnection
	FailureTimeout
	FailureCredentials
	FailureForbidden
	FailureServer
	FailureNoCode
	FailureInvalidCode
)

// Classify says why an attempt failed. A nil error is not a failure and
// reports FailureUnknown, so callers check the error first.
func Classify(err error) Failure {
	switch {
	case err == nil:
		return FailureUnknown
	case errors.Is(err, ErrNoCode):
		return FailureNoCode
	case errors.Is(err, ErrInvalidCode):
		return FailureInvalidCode
	case errors.Is(err, romm.ErrInvalidHostname):
		return FailureHostname
	case errors.Is(err, romm.ErrConnectionRefused):
		return FailureConnection
	case errors.Is(err, romm.ErrTimeout):
		return FailureTimeout
	case errors.Is(err, romm.ErrUnauthorized):
		return FailureCredentials
	case errors.Is(err, romm.ErrForbidden):
		return FailureForbidden
	case errors.Is(err, romm.ErrServerError):
		return FailureServer
	}

	slog.Default().Warn("Unclassified login error", "error", err)
	return FailureUnknown
}

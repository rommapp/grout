package romm

import (
	"errors"
	"net/http"
	"testing"
)

// The login screen decides what to tell the user by asking errors.Is about the
// sentinels, so every status that has a specific meaning has to carry one.
func TestStatusError_Classification(t *testing.T) {
	tests := []struct {
		status int
		want   error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrForbidden},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusConflict, ErrConflict},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrServerError},
		{http.StatusBadGateway, ErrServerError},
	}

	for _, tt := range tests {
		err := statusError(tt.status, []byte("body"))
		if !errors.Is(err, tt.want) {
			t.Errorf("status %d = %v, want it to match %v", tt.status, err, tt.want)
		}
	}
}

// A status with no specific meaning must not be mistaken for one that has one.
func TestStatusError_UnmappedStatusMatchesNothing(t *testing.T) {
	err := statusError(http.StatusBadRequest, nil)

	for _, sentinel := range []error{ErrUnauthorized, ErrForbidden, ErrNotFound, ErrConflict, ErrRateLimited, ErrServerError} {
		if errors.Is(err, sentinel) {
			t.Errorf("status 400 must not match %v", sentinel)
		}
	}
	if err == nil {
		t.Fatal("a non-2xx status is still an error")
	}
}

func TestAPIError_KeepsStatusAndBody(t *testing.T) {
	var apiErr *APIError
	if !errors.As(statusError(404, []byte("no such rom")), &apiErr) {
		t.Fatal("statusError must produce an *APIError")
	}
	if apiErr.StatusCode != 404 || apiErr.Body != "no such rom" {
		t.Errorf("got status %d body %q", apiErr.StatusCode, apiErr.Body)
	}
}

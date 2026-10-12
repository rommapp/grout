package romm

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"
)

// dialError is what net/http returns when connect(2) fails, wrapped the way
// the heartbeat check wraps it before classifying.
func dialError(errno syscall.Errno) error {
	err := &url.Error{
		Op:  "Get",
		URL: "http://192.168.0.25:8080/api/heartbeat",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", errno)},
	}
	return fmt.Errorf("failed to connect: %w", err)
}

// The login screen picks its message with errors.Is, so each dial failure
// that has a specific cause has to come back as that cause's sentinel.
func TestClassifyError_DialFailures(t *testing.T) {
	tests := []struct {
		name  string
		errno syscall.Errno
		want  error
	}{
		{"refused", syscall.ECONNREFUSED, ErrConnectionRefused},
		{"no network on the device", syscall.ENETUNREACH, ErrNetworkUnreachable},
		{"no route to the host", syscall.EHOSTUNREACH, ErrConnectionRefused},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyError(dialError(tt.errno)); !errors.Is(got, tt.want) {
				t.Errorf("ClassifyError(%v) = %v, want it to match %v", tt.errno, got, tt.want)
			}
		})
	}
}

// A device without network is not a host that refused, so the two must not
// be confused.
func TestClassifyError_NetworkUnreachableIsNotRefused(t *testing.T) {
	got := ClassifyError(dialError(syscall.ENETUNREACH))
	if errors.Is(got, ErrConnectionRefused) {
		t.Errorf("ENETUNREACH must not match %v, got %v", ErrConnectionRefused, got)
	}
}

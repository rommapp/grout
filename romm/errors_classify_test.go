package romm

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"
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

// A lookup that did not get an answer is a network that is not ready; only an
// answer saying the name does not exist means the name is wrong. Reporting the
// first as the second sends people to fix a server address that is fine.
func TestClassifyError_OnlyANameThatDoesNotExistIsAnInvalidHostname(t *testing.T) {
	lookup := func(dns *net.DNSError) error {
		return fmt.Errorf("failed to connect: %w", &url.Error{Op: "Get", URL: "https://romm.example/api/heartbeat", Err: dns})
	}

	missing := ClassifyError(lookup(&net.DNSError{Name: "romm.example", IsNotFound: true}))
	if !errors.Is(missing, ErrInvalidHostname) || errors.Is(missing, ErrDNSUnavailable) {
		t.Errorf("a name that does not exist: %v", missing)
	}

	for name, dns := range map[string]*net.DNSError{
		"timed out":                 {Name: "romm.example", IsTimeout: true},
		"temporary":                 {Name: "romm.example", IsTemporary: true},
		"server answered a failure": {Name: "romm.example", Err: "server misbehaving"},
		"no resolver yet":           {Name: "romm.example", Err: "dial udp: connect: network is unreachable"},
	} {
		got := ClassifyError(lookup(dns))
		if !errors.Is(got, ErrDNSUnavailable) || errors.Is(got, ErrInvalidHostname) {
			t.Errorf("%s: %v", name, got)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func clientAnswering(answers ...func() (*http.Response, error)) (*Client, *int) {
	calls := 0
	client := NewClient("https://romm.example")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		answer := answers[len(answers)-1]
		if calls < len(answers) {
			answer = answers[calls]
		}
		calls++
		return answer()
	})}
	return client, &calls
}

func dnsNotReady() (*http.Response, error) {
	return nil, &net.DNSError{Name: "romm.example", Err: "server misbehaving", IsTemporary: true}
}

func serverOK() (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
}

func TestValidateConnectionRetrying_GetsThroughANetworkThatIsStillComingUp(t *testing.T) {
	client, calls := clientAnswering(dnsNotReady, dnsNotReady, serverOK)

	err := client.ValidateConnectionRetrying([]time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}, IsTransient)

	if err != nil || *calls != 3 {
		t.Errorf("err=%v after %d attempts, want success on the third", err, *calls)
	}
}

func TestValidateConnectionRetrying_GivesUpWhenItNeverComesUp(t *testing.T) {
	client, calls := clientAnswering(dnsNotReady)

	err := client.ValidateConnectionRetrying([]time.Duration{time.Millisecond, time.Millisecond}, IsTransient)

	if !errors.Is(err, ErrDNSUnavailable) || *calls != 3 {
		t.Errorf("err=%v after %d attempts, want ErrDNSUnavailable after 3", err, *calls)
	}
}

// A server that refuses the connection is not one that will answer in a second.
func TestValidateConnectionRetrying_DoesNotRetryWhatWaitingWillNotFix(t *testing.T) {
	refused := func() (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	}
	client, calls := clientAnswering(refused)

	err := client.ValidateConnectionRetrying([]time.Duration{time.Millisecond, time.Millisecond}, IsTransient)

	if !errors.Is(err, ErrConnectionRefused) || *calls != 1 {
		t.Errorf("err=%v after %d attempts, want ErrConnectionRefused after 1", err, *calls)
	}
}

// Before a game the wait is short, so a slow host is not waited for again.
func TestValidateConnectionRetrying_TheLaunchPredicateLeavesATimeoutAlone(t *testing.T) {
	timedOut := func() (*http.Response, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}
	}
	client, calls := clientAnswering(timedOut)

	err := client.ValidateConnectionRetrying([]time.Duration{time.Millisecond}, IsNotReadyYet)

	if !errors.Is(err, ErrTimeout) || *calls != 1 {
		t.Errorf("err=%v after %d attempts, want a timeout after 1", err, *calls)
	}
}

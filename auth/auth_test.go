package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"grout/romm"
	"grout/settings"
)

// server stands in for RomM. Each handler is keyed by path so a test only
// declares the endpoints it cares about; anything else is a 404, which is what
// an older or misconfigured server would give.
func server(t *testing.T, handlers map[string]http.HandlerFunc) settings.Host {
	t.Helper()

	mux := http.NewServeMux()
	for path, handler := range handlers {
		mux.HandleFunc(path, handler)
	}

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return settings.Host{RootURI: srv.URL}
}

func json200(body any) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}
}

func status(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(code) }
}

func heartbeat(version string) any {
	return map[string]any{"SYSTEM": map[string]any{"VERSION": version}}
}

func TestConnect_ReportsDevicePairing(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"5.0.0", true},
		{"4.9.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			host := server(t, map[string]http.HandlerFunc{
				"/api/heartbeat": json200(heartbeat(tt.version)),
			})

			capabilities, err := Connect(host)
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}
			if capabilities.SupportsDevicePairing != tt.want {
				t.Errorf("SupportsDevicePairing = %v, want %v for RomM %s",
					capabilities.SupportsDevicePairing, tt.want, tt.version)
			}
		})
	}
}

// A server that answers but will not say what version it is still works. The
// user just gets the pairing code instead of the nicer flow.
func TestConnect_UnreadableHeartbeatFallsBack(t *testing.T) {
	host := server(t, map[string]http.HandlerFunc{
		// ValidateConnection only looks at the status, so a body it cannot
		// parse fails only the call that reads the version out of it.
		"/api/heartbeat": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not json"))
		},
	})

	capabilities, err := Connect(host)
	if err != nil {
		t.Fatalf("Connect must succeed when only the version is unreadable: %v", err)
	}
	if capabilities.SupportsDevicePairing {
		t.Error("an unreadable version must fall back to no device pairing")
	}
}

func TestConnect_UnreachableServer(t *testing.T) {
	host := settings.Host{RootURI: "http://127.0.0.1:1"}

	if _, err := Connect(host); err == nil {
		t.Fatal("expected an error connecting to a closed port")
	}
}

// The status the server returns has to survive as far as Classify, or the
// login screen cannot tell the user which of these went wrong.
func TestConnect_ClassifiesStatus(t *testing.T) {
	tests := []struct {
		status int
		want   Failure
	}{
		{http.StatusUnauthorized, FailureCredentials},
		{http.StatusForbidden, FailureForbidden},
		{http.StatusInternalServerError, FailureServer},
	}

	for _, tt := range tests {
		host := server(t, map[string]http.HandlerFunc{"/api/heartbeat": status(tt.status)})

		_, err := Connect(host)
		if err == nil {
			t.Fatalf("status %d must fail the connection", tt.status)
		}
		if got := Classify(err); got != tt.want {
			t.Errorf("status %d classified as %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestExchangeCode_Success(t *testing.T) {
	host := server(t, map[string]http.HandlerFunc{
		"/api/client-tokens/exchange": json200(romm.TokenExchangeResponse{
			RawToken:  "a-real-token",
			Name:      "grout",
			ExpiresAt: "2030-01-01T00:00:00Z",
			Scopes:    romm.SyncRequiredScopes,
		}),
		"/api/platforms": json200([]romm.Platform{}),
		"/api/users/me":  json200(romm.CurrentUser{Username: "player"}),
	})

	paired, err := ExchangeCode(host, "ABC123")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}

	if paired.Token != "a-real-token" {
		t.Errorf("Token = %q, want the exchanged token", paired.Token)
	}
	if paired.TokenName != "grout" || paired.TokenExpiresAt != "2030-01-01T00:00:00Z" {
		t.Errorf("token details not carried over: name %q expires %q", paired.TokenName, paired.TokenExpiresAt)
	}
	if paired.Username != "player" {
		t.Errorf("Username = %q, want it filled in for the settings screen", paired.Username)
	}
}

func TestExchangeCode_EmptyCode(t *testing.T) {
	if _, err := ExchangeCode(settings.Host{RootURI: "http://example.invalid"}, ""); !errors.Is(err, ErrNoCode) {
		t.Errorf("err = %v, want ErrNoCode without touching the network", err)
	}
}

// A code the server rejects is the most common way this fails, and it is worth
// saying so rather than falling back to "something unexpected happened".
func TestExchangeCode_RejectedCode(t *testing.T) {
	for _, code := range []int{http.StatusNotFound, http.StatusTooManyRequests} {
		host := server(t, map[string]http.HandlerFunc{
			"/api/client-tokens/exchange": status(code),
		})

		_, err := ExchangeCode(host, "WRONG")
		if !errors.Is(err, ErrInvalidCode) {
			t.Errorf("status %d gave %v, want ErrInvalidCode", code, err)
		}
		if got := Classify(err); got != FailureInvalidCode {
			t.Errorf("status %d classified as %v, want FailureInvalidCode", code, got)
		}
	}
}

// Other failures are not the code's fault and must keep their own meaning.
func TestExchangeCode_ServerErrorIsNotACodeProblem(t *testing.T) {
	host := server(t, map[string]http.HandlerFunc{
		"/api/client-tokens/exchange": status(http.StatusInternalServerError),
	})

	_, err := ExchangeCode(host, "ABC123")
	if errors.Is(err, ErrInvalidCode) {
		t.Error("a server error must not be reported as a bad pairing code")
	}
	if got := Classify(err); got != FailureServer {
		t.Errorf("classified as %v, want FailureServer", got)
	}
}

// A code can exchange for a token the server will not then accept. Catching it
// here means the user sees it on the login screen instead of an empty library.
func TestExchangeCode_UnusableToken(t *testing.T) {
	host := server(t, map[string]http.HandlerFunc{
		"/api/client-tokens/exchange": json200(romm.TokenExchangeResponse{RawToken: "stale"}),
		"/api/platforms":              status(http.StatusUnauthorized),
	})

	_, err := ExchangeCode(host, "ABC123")
	if err == nil {
		t.Fatal("expected the token check to fail")
	}
	if got := Classify(err); got != FailureCredentials {
		t.Errorf("classified as %v, want FailureCredentials", got)
	}
}

// The username is a nicety on the settings screen, so failing to read it must
// not cost the user a working login.
func TestExchangeCode_SucceedsWithoutUsername(t *testing.T) {
	host := server(t, map[string]http.HandlerFunc{
		"/api/client-tokens/exchange": json200(romm.TokenExchangeResponse{RawToken: "token"}),
		"/api/platforms":              json200([]romm.Platform{}),
		"/api/users/me":               status(http.StatusInternalServerError),
	})

	paired, err := ExchangeCode(host, "ABC123")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if paired.Token != "token" {
		t.Errorf("Token = %q, want the login to have gone through", paired.Token)
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Failure
	}{
		{"nil", nil, FailureUnknown},
		{"hostname", romm.ErrInvalidHostname, FailureHostname},
		{"refused", romm.ErrConnectionRefused, FailureConnection},
		{"timeout", romm.ErrTimeout, FailureTimeout},
		{"unauthorized", romm.ErrUnauthorized, FailureCredentials},
		{"forbidden", romm.ErrForbidden, FailureForbidden},
		{"server", romm.ErrServerError, FailureServer},
		{"no code", ErrNoCode, FailureNoCode},
		{"bad code", ErrInvalidCode, FailureInvalidCode},
		{"anything else", errors.New("disk on fire"), FailureUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.want {
				t.Errorf("Classify(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

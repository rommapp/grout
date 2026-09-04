package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"grout/romm"
	"grout/settings"
)

// pairingServer answers the token endpoint with each response in turn,
// repeating the last once they run out. Anything else 404s, which is what an
// unexpected call would get from a real server.
func pairingServer(t *testing.T, responses ...http.HandlerFunc) *httptest.Server {
	t.Helper()

	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		i := int(calls.Add(1)) - 1
		if i >= len(responses) {
			i = len(responses) - 1
		}
		responses[i](w, r)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func pending(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, `{"detail":"authorization_pending"}`)
}

func approved(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, `{"access_token":"tok-abc","device_id":"dev-1",`+
		`"scopes":["assets.read","assets.write","devices.read","devices.write"],`+
		`"expires_at":"2027-01-01T00:00:00Z"}`)
}

func denied(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, `{"detail":"access_denied"}`)
}

// testPairing is a request already open against url, polling fast enough that
// tests do not wait on it.
func testPairing(url string) *Pairing {
	return &Pairing{
		client:     romm.NewClient(url),
		host:       settings.Host{RootURI: url},
		deviceName: "Test Device",
		deviceCode: "dc-123",
		interval:   time.Millisecond,
		slowDown:   30 * time.Millisecond,
		expiresAt:  time.Now().Add(time.Minute),
	}
}

func TestPoll_PendingThenApproved(t *testing.T) {
	pairing := testPairing(pairingServer(t, pending, approved).URL)

	outcome, token, err := pairing.poll(context.Background())
	if outcome != PairingApproved || err != nil {
		t.Fatalf("outcome = %v, err = %v, want PairingApproved", outcome, err)
	}
	if token.AccessToken != "tok-abc" || token.DeviceID != "dev-1" {
		t.Errorf("unexpected token: %+v", token)
	}
}

func TestPoll_Denied(t *testing.T) {
	pairing := testPairing(pairingServer(t, denied).URL)

	if outcome, _, _ := pairing.poll(context.Background()); outcome != PairingDenied {
		t.Errorf("outcome = %v, want PairingDenied", outcome)
	}
}

// Cancelling has to stop the poll promptly, since it is what the B button on
// the pairing screen does.
func TestPoll_StopsWhenCancelled(t *testing.T) {
	pairing := testPairing(pairingServer(t, pending).URL)
	pairing.interval = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan PairingOutcome, 1)
	go func() {
		outcome, _, _ := pairing.poll(ctx)
		done <- outcome
	}()

	select {
	case outcome := <-done:
		if outcome != PairingCancelled {
			t.Errorf("outcome = %v, want PairingCancelled", outcome)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("poll kept running after its context was cancelled")
	}
}

func TestPoll_ExpiresLocally(t *testing.T) {
	pairing := testPairing(pairingServer(t, pending).URL)
	pairing.expiresAt = time.Now().Add(-time.Second)

	if outcome, _, _ := pairing.poll(context.Background()); outcome != PairingExpired {
		t.Errorf("outcome = %v, want PairingExpired", outcome)
	}
}

// Wifi on these devices drops often enough that one failed poll is not a
// reason to give up, but a server that has stopped answering is.
func TestPoll_GivesUpAfterRepeatedNetworkErrors(t *testing.T) {
	srv := pairingServer(t, pending)
	srv.Close()

	pairing := testPairing(srv.URL)

	outcome, _, err := pairing.poll(context.Background())
	if outcome != PairingFailed {
		t.Fatalf("outcome = %v, want PairingFailed", outcome)
	}
	if err == nil {
		t.Error("the error that ended the attempt must be reported")
	}
}

// A server asking grout to slow down must actually slow it down, or the next
// poll is just as unwelcome as the last.
func TestPoll_SlowDownBacksOff(t *testing.T) {
	slowDown := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"detail":"slow_down"}`)
	}

	pairing := testPairing(pairingServer(t, slowDown, approved).URL)
	pairing.expiresAt = time.Now().Add(time.Hour)

	start := time.Now()
	outcome, _, _ := pairing.poll(context.Background())

	if outcome != PairingApproved {
		t.Fatalf("outcome = %v, want PairingApproved", outcome)
	}
	if elapsed := time.Since(start); elapsed < pairing.slowDown {
		t.Errorf("second poll came after %v, want at least the %v back-off", elapsed, pairing.slowDown)
	}
}

// A server that names no interval must not be polled as fast as the loop can
// go, and one that names no lifetime must not expire before the user has a
// chance to answer.
func TestStartPairing_DefaultsWhenServerOmitsTiming(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/device/init", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"device_code":"dc-1","user_code":"ABCD","verification_path_complete":"/pair?user_code=ABCD"}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	pairing, err := StartPairing(settings.Host{RootURI: srv.URL}, "Test Device")
	if err != nil {
		t.Fatalf("StartPairing: %v", err)
	}

	if pairing.slowDown != slowDownStep {
		t.Errorf("slowDown = %v, want the %v default", pairing.slowDown, slowDownStep)
	}
	if pairing.interval != defaultPollInterval {
		t.Errorf("interval = %v, want the %v default", pairing.interval, defaultPollInterval)
	}
	if remaining := time.Until(pairing.expiresAt); remaining < time.Minute {
		t.Errorf("expires in %v, want a usable window", remaining)
	}
	if want := srv.URL + "/pair?user_code=ABCD"; pairing.VerificationURL != want {
		t.Errorf("VerificationURL = %q, want %q", pairing.VerificationURL, want)
	}
}

// The identifier is what lets the server recognise this device on a later
// pairing, so one it already has must be kept.
func TestStartPairing_KeepsExistingDeviceID(t *testing.T) {
	var sent romm.DeviceAuthInitRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/device/init", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&sent)
		fmt.Fprint(w, `{"device_code":"dc-1","expires_in":300,"interval":5}`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	host := settings.Host{RootURI: srv.URL, ClientDeviceID: "known-device"}
	if _, err := StartPairing(host, "Test Device"); err != nil {
		t.Fatalf("StartPairing: %v", err)
	}

	if sent.ClientDeviceIdentifier != "known-device" {
		t.Errorf("sent identifier %q, want the stored one", sent.ClientDeviceIdentifier)
	}

	if _, err := StartPairing(settings.Host{RootURI: srv.URL}, "Test Device"); err != nil {
		t.Fatalf("StartPairing: %v", err)
	}
	if sent.ClientDeviceIdentifier == "" || sent.ClientDeviceIdentifier == "known-device" {
		t.Errorf("sent identifier %q, want a freshly generated one", sent.ClientDeviceIdentifier)
	}
}

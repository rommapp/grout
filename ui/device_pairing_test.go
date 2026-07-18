package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"grout/romm"
)

// warmUpServer answers GET /api/platforms with 403 for the first failFirst
// calls, then 200 with an empty platform list, counting every call.
func warmUpServer(failFirst int, calls *int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(calls, 1))
		if n <= failFirst {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"detail":"Forbidden"}`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
}

func TestWarmUpToken_RetriesThenStopsOnSuccess(t *testing.T) {
	var calls int32
	srv := warmUpServer(2, &calls) // 403, 403, then 200
	defer srv.Close()

	warmUpTokenWith(romm.NewClient(srv.URL), &atomic.Bool{}, 6, time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (two 403s then success, no further retries)", got)
	}
}

func TestWarmUpToken_ExhaustsAttempts(t *testing.T) {
	var calls int32
	srv := warmUpServer(100, &calls) // always 403
	defer srv.Close()

	warmUpTokenWith(romm.NewClient(srv.URL), &atomic.Bool{}, 4, time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 4 {
		t.Errorf("calls = %d, want 4 (bounded by attempts)", got)
	}
}

func TestWarmUpToken_StopsWhenCancelled(t *testing.T) {
	var calls int32
	srv := warmUpServer(100, &calls) // always 403
	defer srv.Close()

	cancelled := &atomic.Bool{}
	cancelled.Store(true)
	warmUpTokenWith(romm.NewClient(srv.URL), cancelled, 6, time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("calls = %d, want 0 (cancelled before first request)", got)
	}
}

// pairingTestServer answers /api/auth/device/token with each response in
// sequence, repeating the last one once exhausted.
func pairingTestServer(t *testing.T, responses ...func(w http.ResponseWriter)) *httptest.Server {
	t.Helper()
	var calls int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/auth/device/token" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		i := int(atomic.AddInt32(&calls, 1)) - 1
		if i >= len(responses) {
			i = len(responses) - 1
		}
		responses[i](w)
	}))
}

func pending(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, `{"detail":"authorization_pending"}`)
}

func approved(w http.ResponseWriter) {
	fmt.Fprint(w, `{"access_token":"tok-abc","device_id":"dev-1","scopes":["assets.read","assets.write","devices.read","devices.write"],"expires_at":"2027-01-01T00:00:00Z"}`)
}

func denied(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprint(w, `{"detail":"access_denied"}`)
}

func testInitResp() *romm.DeviceAuthInitResponse {
	return &romm.DeviceAuthInitResponse{
		DeviceCode: "dc-123",
		UserCode:   "ABCD-1234",
		ExpiresIn:  30,
		Interval:   0, // 0 → loop clamps to a floor; tests override via pollTick
	}
}

func TestPollLoop_PendingThenSuccess(t *testing.T) {
	srv := pairingTestServer(t, pending, approved)
	defer srv.Close()

	s := NewDevicePairingScreen()
	res := s.poll(romm.NewClient(srv.URL), testInitResp(), &atomic.Bool{}, 10*time.Millisecond)
	if res.Outcome != DevicePairingSuccess {
		t.Fatalf("outcome = %v, want DevicePairingSuccess", res.Outcome)
	}
	if res.Token == nil || res.Token.AccessToken != "tok-abc" || res.Token.DeviceID != "dev-1" {
		t.Errorf("unexpected token: %+v", res.Token)
	}
}

func TestPollLoop_Denied(t *testing.T) {
	srv := pairingTestServer(t, denied)
	defer srv.Close()

	s := NewDevicePairingScreen()
	res := s.poll(romm.NewClient(srv.URL), testInitResp(), &atomic.Bool{}, 10*time.Millisecond)
	if res.Outcome != DevicePairingDenied {
		t.Fatalf("outcome = %v, want DevicePairingDenied", res.Outcome)
	}
}

func TestPollLoop_Cancelled(t *testing.T) {
	srv := pairingTestServer(t, pending)
	defer srv.Close()

	cancelled := &atomic.Bool{}
	cancelled.Store(true)

	s := NewDevicePairingScreen()
	done := make(chan pollResult, 1)
	go func() {
		done <- s.poll(romm.NewClient(srv.URL), testInitResp(), cancelled, 10*time.Millisecond)
	}()
	select {
	case res := <-done:
		if res.Outcome != DevicePairingCancelled {
			t.Fatalf("outcome = %v, want DevicePairingCancelled", res.Outcome)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("poll did not stop after cancellation")
	}
}

func TestPollLoop_ExpiresLocally(t *testing.T) {
	srv := pairingTestServer(t, pending)
	defer srv.Close()

	init := testInitResp()
	init.ExpiresIn = 0 // deadline already passed on first check

	s := NewDevicePairingScreen()
	res := s.poll(romm.NewClient(srv.URL), init, &atomic.Bool{}, 10*time.Millisecond)
	if res.Outcome != DevicePairingExpired {
		t.Fatalf("outcome = %v, want DevicePairingExpired", res.Outcome)
	}
}

func TestPollLoop_ThreeNetworkErrorsFail(t *testing.T) {
	srv := pairingTestServer(t, pending)
	srv.Close() // immediately unreachable → connection errors

	s := NewDevicePairingScreen()
	res := s.poll(romm.NewClient(srv.URL), testInitResp(), &atomic.Bool{}, 10*time.Millisecond)
	if res.Outcome != DevicePairingFailed {
		t.Fatalf("outcome = %v, want DevicePairingFailed", res.Outcome)
	}
	if res.Err == nil {
		t.Error("expected the final network error to be returned")
	}
}

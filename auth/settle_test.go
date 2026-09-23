package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"grout/romm"
)

// settleServer refuses the first refusals reads with a 403, the way RomM does
// in the moment right after issuing a token, then starts answering.
func settleServer(t *testing.T, refusals int, calls *atomic.Int32) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if int(calls.Add(1)) <= refusals {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, `{"detail":"Forbidden"}`)
			return
		}
		fmt.Fprint(w, `[]`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSettleToken_StopsOnceTheTokenWorks(t *testing.T) {
	var calls atomic.Int32
	srv := settleServer(t, 2, &calls)

	settleTokenWith(context.Background(), romm.NewClient(srv.URL), 6, time.Millisecond)

	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3: two refusals then a success, and no retry after it", got)
	}
}

// A token that never works is a real permission problem, left for the next
// screen to report rather than retried forever here.
func TestSettleToken_GivesUp(t *testing.T) {
	var calls atomic.Int32
	srv := settleServer(t, 100, &calls)

	settleTokenWith(context.Background(), romm.NewClient(srv.URL), 4, time.Millisecond)

	if got := calls.Load(); got != 4 {
		t.Errorf("calls = %d, want it bounded at the 4 attempts asked for", got)
	}
}

func TestSettleToken_StopsWhenCancelled(t *testing.T) {
	var calls atomic.Int32
	srv := settleServer(t, 100, &calls)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	settleTokenWith(ctx, romm.NewClient(srv.URL), 6, time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Errorf("calls = %d, want none once cancelled", got)
	}
}

// Cancelling during the wait between attempts has to take effect too, not just
// before the first one.
func TestSettleToken_StopsWhileWaiting(t *testing.T) {
	var calls atomic.Int32
	srv := settleServer(t, 100, &calls)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	done := make(chan struct{})
	go func() {
		settleTokenWith(ctx, romm.NewClient(srv.URL), 100, 50*time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("settleToken kept retrying after its context was cancelled")
	}
}

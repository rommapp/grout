package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/rommapp/grout/core/romm"
)

var epoch = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

type fakeTiming struct {
	mu     sync.Mutex
	sleeps []time.Duration
	now    time.Time
	err    error
}

func (f *fakeTiming) timing() Timing {
	t := DefaultTiming()
	t.Now = func() time.Time { return f.now }
	t.Sleep = func(ctx context.Context, d time.Duration) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.sleeps = append(f.sleeps, d)
		return f.err
	}
	return t
}

type scripted struct {
	mu    sync.Mutex
	steps []func(w http.ResponseWriter)
	polls int
}

func (s *scripted) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		i := s.polls
		s.polls++
		s.mu.Unlock()
		if i >= len(s.steps) {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.steps[i](w)
	})
}

func state(name string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail":"` + name + `"}`))
	}
}

func status(code int) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) { w.WriteHeader(code) }
}

func approve(w http.ResponseWriter) {
	w.Write([]byte(`{"access_token":"tok","device_id":"dev","scopes":["me.read"],"expires_at":"2027-01-01T00:00:00Z"}`))
}

func newClient(t *testing.T, h http.Handler) *romm.Client {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return romm.New(romm.Options{BaseURL: srv.URL})
}

func pending() Pending {
	return Pending{DeviceCode: "dc", Interval: 5 * time.Second, ExpiresAt: epoch.Add(15 * time.Minute)}
}

func TestStartBuildsVerificationURLAndDefaults(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"device_code":"dc","user_code":"ABCD-2345","verification_path_complete":"/pair/device?code=ABCD2345","expires_in":0,"interval":0}`))
	}))
	f := &fakeTiming{now: epoch}
	p, err := Start(context.Background(), c, "https://romm.example/", romm.DeviceInitRequest{}, f.timing())
	if err != nil {
		t.Fatal(err)
	}
	if p.VerificationURL != "https://romm.example/pair/device?code=ABCD2345" || p.DeviceCode != "dc" || p.UserCode != "ABCD-2345" {
		t.Fatalf("%+v", p)
	}
	if p.Interval != 5*time.Second || !p.ExpiresAt.Equal(epoch.Add(15*time.Minute)) {
		t.Fatalf("%+v", p)
	}
}

func TestStartUsesServerValues(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"device_code":"dc","expires_in":60,"interval":2}`))
	}))
	f := &fakeTiming{now: epoch}
	p, err := Start(context.Background(), c, "https://romm.example", romm.DeviceInitRequest{}, f.timing())
	if err != nil || p.Interval != 2*time.Second || !p.ExpiresAt.Equal(epoch.Add(time.Minute)) {
		t.Fatalf("%+v %v", p, err)
	}
}

func TestStartReturnsError(t *testing.T) {
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }))
	f := &fakeTiming{now: epoch}
	if _, err := Start(context.Background(), c, "https://x", romm.DeviceInitRequest{}, f.timing()); err == nil {
		t.Fatal("expected error")
	}
}

func TestWaitApprovedAfterPending(t *testing.T) {
	s := &scripted{steps: []func(http.ResponseWriter){state("authorization_pending"), state("authorization_pending"), approve}}
	f := &fakeTiming{now: epoch}
	a, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeApproved || a.Token != "tok" || a.DeviceID != "dev" || a.ExpiresAt != "2027-01-01T00:00:00Z" || !reflect.DeepEqual(a.Scopes, []string{"me.read"}) {
		t.Fatalf("%+v %v %v", a, o, err)
	}
	want := []time.Duration{5 * time.Second, 5 * time.Second, 5 * time.Second}
	if s.polls != 3 || !reflect.DeepEqual(f.sleeps, want) {
		t.Fatalf("polls=%d sleeps=%v", s.polls, f.sleeps)
	}
}

func TestWaitSlowDownAddsFiveSeconds(t *testing.T) {
	s := &scripted{steps: []func(http.ResponseWriter){state("authorization_pending"), state("slow_down"), state("slow_down"), approve}}
	f := &fakeTiming{now: epoch}
	_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeApproved {
		t.Fatalf("%v %v", o, err)
	}
	want := []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second}
	if !reflect.DeepEqual(f.sleeps, want) {
		t.Fatalf("sleeps=%v", f.sleeps)
	}
}

func TestWaitRateLimitedSlowsDown(t *testing.T) {
	s := &scripted{steps: []func(http.ResponseWriter){status(http.StatusTooManyRequests), approve}}
	f := &fakeTiming{now: epoch}
	_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeApproved {
		t.Fatalf("%v %v", o, err)
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second}
	if !reflect.DeepEqual(f.sleeps, want) {
		t.Fatalf("sleeps=%v", f.sleeps)
	}
}

func TestWaitRateLimitedDoesNotCountAsNetworkError(t *testing.T) {
	steps := []func(http.ResponseWriter){status(500), status(500), status(429), status(500), status(500), approve}
	s := &scripted{steps: steps}
	f := &fakeTiming{now: epoch}
	_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeApproved {
		t.Fatalf("%v %v", o, err)
	}
}

func TestWaitDeniedAndExpired(t *testing.T) {
	cases := map[string]Outcome{"access_denied": OutcomeDenied, "expired_token": OutcomeExpired}
	for name, want := range cases {
		s := &scripted{steps: []func(http.ResponseWriter){state(name)}}
		f := &fakeTiming{now: epoch}
		_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
		if err != nil || o != want {
			t.Fatalf("%s: %v %v", name, o, err)
		}
	}
}

func TestWaitExpiresLocallyWithoutPolling(t *testing.T) {
	s := &scripted{}
	f := &fakeTiming{now: epoch.Add(time.Hour)}
	_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeExpired || s.polls != 0 {
		t.Fatalf("%v %v polls=%d", o, err, s.polls)
	}
}

func TestWaitThreeNetworkErrorsFail(t *testing.T) {
	s := &scripted{steps: []func(http.ResponseWriter){status(500), status(500), status(500), approve}}
	f := &fakeTiming{now: epoch}
	_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if o != OutcomeFailed || err == nil || s.polls != 3 {
		t.Fatalf("%v %v polls=%d", o, err, s.polls)
	}
}

func TestWaitStateResetsNetworkErrorCount(t *testing.T) {
	steps := []func(http.ResponseWriter){status(500), status(500), state("authorization_pending"), status(500), status(500), approve}
	s := &scripted{steps: steps}
	f := &fakeTiming{now: epoch}
	_, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeApproved {
		t.Fatalf("%v %v", o, err)
	}
}

func TestWaitCancelledDuringSleep(t *testing.T) {
	s := &scripted{}
	f := &fakeTiming{now: epoch, err: context.Canceled}
	_, _, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if !errors.Is(err, context.Canceled) || s.polls != 0 {
		t.Fatalf("%v polls=%d", err, s.polls)
	}
}

func TestWaitKeepsApprovalWhenCancelledAfterPoll(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	step := func(w http.ResponseWriter) {
		cancel()
		time.Sleep(20 * time.Millisecond)
		approve(w)
	}
	s := &scripted{steps: []func(http.ResponseWriter){step}}
	f := &fakeTiming{now: epoch}
	a, o, err := Wait(ctx, newClient(t, s.handler()), pending(), f.timing())
	if err != nil || o != OutcomeApproved || a.Token != "tok" {
		t.Fatalf("%+v %v %v", a, o, err)
	}
}

func TestOutcomesListsEveryOutcome(t *testing.T) {
	want := []Outcome{OutcomeApproved, OutcomeDenied, OutcomeExpired, OutcomeFailed}
	if !reflect.DeepEqual(Outcomes(), want) {
		t.Fatalf("%v", Outcomes())
	}
}

func TestDefaultTiming(t *testing.T) {
	d := DefaultTiming()
	if d.SlowDownStep != 5*time.Second || d.MaxNetErrors != 3 || d.DefaultEvery != 5*time.Second || d.DefaultLife != 15*time.Minute || d.PollTimeout != 30*time.Second || d.Now == nil || d.Sleep == nil {
		t.Fatalf("%+v", d)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.Sleep(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	if err := d.Sleep(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestWaitZeroPollTimeoutUsesDefault(t *testing.T) {
	s := &scripted{steps: []func(http.ResponseWriter){approve}}
	f := &fakeTiming{now: epoch}
	tm := Timing{Sleep: f.timing().Sleep, Now: f.timing().Now}
	a, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), tm)
	if err != nil || o != OutcomeApproved || a.Token != "tok" || s.polls != 1 {
		t.Fatalf("%+v %v %v polls=%d", a, o, err, s.polls)
	}
}

func TestZeroTimingFieldsUseDefaults(t *testing.T) {
	got := Timing{}.withDefaults()
	want := DefaultTiming()
	if got.Sleep == nil || got.Now == nil {
		t.Fatal("function fields not defaulted")
	}
	got.Sleep, got.Now, want.Sleep, want.Now = nil, nil, nil, nil
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v != %+v", got, want)
	}
	neg := Timing{SlowDownStep: -1, MaxNetErrors: -2, PollTimeout: -3}.withDefaults()
	if neg.SlowDownStep != want.SlowDownStep || neg.MaxNetErrors != want.MaxNetErrors || neg.PollTimeout != want.PollTimeout {
		t.Fatalf("%+v", neg)
	}
}

func TestWaitApprovedWithoutTokenFails(t *testing.T) {
	empty := func(w http.ResponseWriter) {
		w.Write([]byte(`{"access_token":"","device_id":"dev","expires_at":"2027-01-01T00:00:00Z"}`))
	}
	s := &scripted{steps: []func(http.ResponseWriter){empty}}
	f := &fakeTiming{now: epoch}
	a, o, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err == nil || o != OutcomeFailed || a.Token != "" || a.DeviceID != "" {
		t.Fatalf("%+v %v %v", a, o, err)
	}
}

func TestWaitNormalizesExpiry(t *testing.T) {
	reply := func(w http.ResponseWriter) {
		w.Write([]byte(`{"access_token":"tok","expires_at":"2027-01-01T00:00:00.123456+00:00"}`))
	}
	s := &scripted{steps: []func(http.ResponseWriter){reply}}
	f := &fakeTiming{now: epoch}
	a, _, err := Wait(context.Background(), newClient(t, s.handler()), pending(), f.timing())
	if err != nil || a.ExpiresAt != "2027-01-01T00:00:00Z" {
		t.Fatalf("%+v %v", a, err)
	}
}

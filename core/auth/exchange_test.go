package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestNormalizeCode(t *testing.T) {
	for _, in := range []string{" abcd-2345 ", "abcd 2345"} {
		if got := NormalizeCode(in); got != "ABCD2345" {
			t.Fatalf("%q -> %q", in, got)
		}
	}
}

func TestExchangeErrors(t *testing.T) {
	var body string
	requests := 0
	code := http.StatusNotFound
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		if code == http.StatusOK {
			w.Write([]byte(`{"raw_token":"rt","device_id":"d"}`))
			return
		}
		w.WriteHeader(code)
	}))
	if _, err := Exchange(context.Background(), c, "  ", Timing{}); !errors.Is(err, ErrNoCode) || requests != 0 {
		t.Fatalf("%v requests=%d", err, requests)
	}
	if _, err := Exchange(context.Background(), c, "abcd-2345", Timing{}); !errors.Is(err, ErrInvalidCode) || body != `{"code":"ABCD2345"}` {
		t.Fatalf("%v body=%s", err, body)
	}
	code = http.StatusTooManyRequests
	if _, err := Exchange(context.Background(), c, "abcd-2345", Timing{}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("%v", err)
	}
	code = http.StatusInternalServerError
	if _, err := Exchange(context.Background(), c, "abcd-2345", Timing{}); err == nil || errors.Is(err, ErrInvalidCode) || errors.Is(err, ErrRateLimited) {
		t.Fatalf("%v", err)
	}
	code = http.StatusOK
	tok, err := Exchange(context.Background(), c, "abcd-2345", Timing{})
	if err != nil || tok.RawToken != "rt" {
		t.Fatalf("%+v %v", tok, err)
	}
}

func TestNormalizeExpiry(t *testing.T) {
	for in, want := range map[string]string{
		"":                                 "",
		"never":                            "",
		"2027-01-01T00:00:00Z":             "2027-01-01T00:00:00Z",
		"2027-01-01T00:00:00+00:00":        "2027-01-01T00:00:00Z",
		"2027-01-01T00:00:00.123456+00:00": "2027-01-01T00:00:00Z",
		"2027-01-01T02:30:00.5+02:00":      "2027-01-01T00:30:00Z",
		"2027-01-01T00:00:00.123456":       "2027-01-01T00:00:00Z",
		"2027-01-01T00:00:00":              "2027-01-01T00:00:00Z",
	} {
		if got := NormalizeExpiry(in); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
}

func TestExchangeSurvivesCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cancel()
		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(`{"raw_token":"rt","expires_at":"2027-01-01T00:00:00.5+00:00"}`))
	}))
	tok, err := Exchange(ctx, c, "abcd-2345", Timing{})
	if err != nil || tok.RawToken != "rt" || tok.ExpiresAt != "2027-01-01T00:00:00Z" {
		t.Fatalf("%+v %v", tok, err)
	}
}

package romm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStatusErrorAndUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte("nope"))
	}))
	defer srv.Close()
	_, err := New(Options{BaseURL: srv.URL}).Heartbeat(context.Background())
	var se *StatusError
	if !errors.As(err, &se) || se.Code != 401 || se.Body != "nope" || !IsUnauthorized(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestForbiddenIsUnauthorizedButServerErrorIsNot(t *testing.T) {
	if !IsUnauthorized(&StatusError{Code: 403}) || IsUnauthorized(&StatusError{Code: 500}) || IsUnauthorized(errors.New("x")) {
		t.Fatal("IsUnauthorized")
	}
}

func TestStatusErrorBodyIsCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(strings.Repeat("x", 10000)))
	}))
	defer srv.Close()
	_, err := New(Options{BaseURL: srv.URL}).Heartbeat(context.Background())
	var se *StatusError
	if !errors.As(err, &se) || len(se.Body) != 4096 {
		t.Fatalf("err=%v", err)
	}
}

func TestSendEncodesQuery(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	var out map[string]any
	err := New(Options{BaseURL: srv.URL}).getJSON(context.Background(), "/api/roms", url.Values{"platform_ids": {"3"}, "search_term": {"a b"}}, &out)
	if err != nil || got != "platform_ids=3&search_term=a+b" {
		t.Fatalf("query=%q err=%v", got, err)
	}
}

func TestNoAuthHeaderWhenEmpty(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["Authorization"]
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	New(Options{BaseURL: srv.URL}).Heartbeat(context.Background())
	if present {
		t.Fatal("empty Authorization header sent")
	}
}

func TestInsecureSkipVerifyReachesTLSServer(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"SAVE_SYNC":{"SNAPSHOTS":true}}`))
	}))
	defer srv.Close()
	if _, err := New(Options{BaseURL: srv.URL}).Heartbeat(context.Background()); err == nil {
		t.Fatal("self-signed cert accepted without InsecureSkipVerify")
	}
	if _, err := New(Options{BaseURL: srv.URL, InsecureSkipVerify: true}).Heartbeat(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestContextCancelStopsRequest(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := New(Options{BaseURL: srv.URL}).Heartbeat(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
}

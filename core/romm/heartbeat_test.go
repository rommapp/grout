package romm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHeartbeatSnapshots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/heartbeat" || r.Header.Get("Authorization") != "Bearer t" {
			w.WriteHeader(404)
			return
		}
		w.Write([]byte(`{"SYSTEM":{"VERSION":"5.5.0"},"SAVE_SYNC":{"SNAPSHOTS":true}}`))
	}))
	defer srv.Close()
	h, err := New(Options{BaseURL: srv.URL + "/", AuthHeader: "Bearer t"}).Heartbeat(context.Background())
	if err != nil || h.System.Version != "5.5.0" || h.RequireSnapshots() != nil {
		t.Fatalf("h=%+v err=%v", h, err)
	}
}

func TestHeartbeatWithoutSaveSyncIsTooOld(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"SYSTEM":{"VERSION":"4.9.2"}}`))
	}))
	defer srv.Close()
	h, err := New(Options{BaseURL: srv.URL}).Heartbeat(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(h.RequireSnapshots(), ErrServerTooOld) {
		t.Fatal("expected ErrServerTooOld")
	}
}

func TestHeartbeatSnapshotsFalseIsTooOld(t *testing.T) {
	h := Heartbeat{}
	h.SaveSync.Snapshots = false
	if !errors.Is(h.RequireSnapshots(), ErrServerTooOld) {
		t.Fatal("expected ErrServerTooOld")
	}
}

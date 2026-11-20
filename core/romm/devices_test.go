package romm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterDeviceAcceptsExisting(t *testing.T) {
	for _, tc := range []struct {
		status int
		id     string
	}{{200, "old"}, {201, "new"}} {
		var got map[string]any
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/devices" || r.Method != http.MethodPost {
				t.Errorf("%s %s", r.Method, r.URL.Path)
			}
			json.NewDecoder(r.Body).Decode(&got)
			w.WriteHeader(tc.status)
			w.Write([]byte(`{"device_id":"` + tc.id + `","name":"Pad"}`))
		}))
		ref, err := New(Options{BaseURL: srv.URL}).RegisterDevice(context.Background(), DeviceRegistration{Name: "Pad", SyncMode: SyncModeAPI})
		srv.Close()
		if err != nil || ref.DeviceID != tc.id || ref.Name != "Pad" || got["sync_mode"] != "api" {
			t.Fatalf("status %d ref=%+v got=%v err=%v", tc.status, ref, got, err)
		}
	}
}

func TestUpdateDevicePutsOnlySetFields(t *testing.T) {
	var method, path, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	err := New(Options{BaseURL: srv.URL}).UpdateDevice(context.Background(), "d1", DeviceUpdate{ClientVersion: "3.0.0"})
	if err != nil || method != http.MethodPut || path != "/api/devices/d1" || body != `{"client_version":"3.0.0"}` {
		t.Fatalf("method=%s path=%s body=%s err=%v", method, path, body, err)
	}
}

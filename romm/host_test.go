package romm

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewClientDeviceID(t *testing.T) {
	a := NewClientDeviceID()
	b := NewClientDeviceID()
	if len(a) != 32 {
		t.Errorf("len = %d, want 32 hex chars", len(a))
	}
	if a == b {
		t.Error("two generated IDs must differ")
	}
}

func TestHostClientDeviceIDRoundTrip(t *testing.T) {
	h := Host{ClientDeviceID: "abc123"}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	var got Host
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ClientDeviceID != "abc123" {
		t.Errorf("ClientDeviceID = %q, want abc123", got.ClientDeviceID)
	}
}

func TestAuthHeaderTokenOnly(t *testing.T) {
	h := Host{Token: "tok-abc"}
	if got := h.AuthHeader(); got != "Bearer tok-abc" {
		t.Errorf("AuthHeader() = %q, want Bearer tok-abc", got)
	}
	if got := (Host{}).AuthHeader(); got != "" {
		t.Errorf("AuthHeader() on empty host = %q, want empty string", got)
	}
}

func TestHostIgnoresLegacyPassword(t *testing.T) {
	// Configs written before the RomM 5.0 cutover contain a password field;
	// loading one must neither fail nor resurrect basic auth.
	data := []byte(`{"root_uri":"http://romm.local","username":"u","password":"secret","token":"tok"}`)
	var h Host
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatalf("unmarshal legacy config: %v", err)
	}
	if got := h.AuthHeader(); got != "Bearer tok" {
		t.Errorf("AuthHeader() = %q, want Bearer tok", got)
	}
	out, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "secret") {
		t.Error("legacy password must not survive a config round-trip")
	}
}

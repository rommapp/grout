package romm

import (
	"encoding/json"
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

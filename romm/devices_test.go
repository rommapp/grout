package romm

import (
	"encoding/json"
	"testing"
)

func TestRegisterDeviceRequest_IncludesSyncMode(t *testing.T) {
	req := RegisterDeviceRequest{
		Name:          "Handheld",
		Platform:      "muOS",
		Client:        "grout",
		ClientVersion: "4.9.0.0",
		SyncMode:      "api",
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["sync_mode"] != "api" {
		t.Errorf("sync_mode = %v, want api", got["sync_mode"])
	}
}

package romm

import (
	"encoding/json"
	"testing"
)

func TestSyncNegotiatePayload_JSON(t *testing.T) {
	p := SyncNegotiatePayload{
		DeviceID: "dev-1",
		Saves: []ClientSaveState{
			{RomID: 7, FileName: "game.srm", Slot: "autosave", Emulator: "mgba", ContentHash: "abc", FileSizeBytes: 42},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["device_id"] != "dev-1" {
		t.Errorf("device_id = %v", got["device_id"])
	}
	saves := got["saves"].([]any)
	first := saves[0].(map[string]any)
	if first["rom_id"].(float64) != 7 {
		t.Errorf("rom_id = %v", first["rom_id"])
	}
	if first["file_name"] != "game.srm" {
		t.Errorf("file_name = %v", first["file_name"])
	}
}

func TestSyncNegotiateResponse_Decode(t *testing.T) {
	raw := `{
		"session_id": 99,
		"operations": [
			{"action":"download","rom_id":7,"save_id":12,"file_name":"game.srm","slot":"autosave","reason":"server newer","server_updated_at":"2025-06-01T00:00:00Z"}
		],
		"total_upload":0,"total_download":1,"total_conflict":0,"total_no_op":0
	}`
	var resp SyncNegotiateResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.SessionID != 99 {
		t.Fatalf("session_id = %d", resp.SessionID)
	}
	if len(resp.Operations) != 1 {
		t.Fatalf("ops = %d", len(resp.Operations))
	}
	op := resp.Operations[0]
	if op.Action != "download" || op.RomID != 7 || op.SaveID == nil || *op.SaveID != 12 {
		t.Errorf("bad op: %+v", op)
	}
	if op.Slot == nil || *op.Slot != "autosave" {
		t.Errorf("slot = %v", op.Slot)
	}
}

package romm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sonh/qs"
)

func TestSaveQueryDeviceOnlyValidityAndEncoding(t *testing.T) {
	query := SaveQuery{DeviceID: "device-42"}
	if !query.Valid() {
		t.Fatal("device-only save query must be valid")
	}
	for name, query := range map[string]SaveQuery{
		"zero":     {},
		"emulator": {Emulator: "mgba"},
		"slot":     {Slot: "autosave"},
	} {
		t.Run(name, func(t *testing.T) {
			if query.Valid() {
				t.Fatalf("unscoped query must remain invalid: %+v", query)
			}
		})
	}

	values, err := qs.NewEncoder().Values(query)
	if err != nil {
		t.Fatal(err)
	}
	if got := values.Encode(); got != "device_id=device-42" {
		t.Fatalf("encoded device-only query = %q, want exact device_id", got)
	}
}

func TestGetSavesDeviceOnlyMakesOneExactRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != endpointSaves {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("device_id") != "device-42" || query.Has("rom_id") || query.Has("platform_id") {
			t.Errorf("query = %q, want device_id only", r.URL.RawQuery)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test" {
			t.Errorf("authorization = %q", got)
		}
		_ = json.NewEncoder(w).Encode([]Save{{ID: 1, RomID: 7}})
	}))
	defer server.Close()

	saves, err := NewClient(server.URL, WithAuthHeader("Bearer test")).GetSaves(SaveQuery{DeviceID: "device-42"})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(saves) != 1 || saves[0].ID != 1 {
		t.Fatalf("requests=%d saves=%+v", requests, saves)
	}
}

// optimistic=false must be transmitted, not dropped by omitempty — the server defaults
// it to true, which would mark the device synced before the file is written.
func TestSaveContentQuery_OptimisticFalseIsEncoded(t *testing.T) {
	v, err := qs.NewEncoder().Values(SaveContentQuery{DeviceID: "dev-1", Optimistic: false})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Encode(); !strings.Contains(got, "optimistic=false") {
		t.Errorf("expected optimistic=false in query, got %q", got)
	}
}

func TestSaveContentQuery_OptimisticTrueIsEncoded(t *testing.T) {
	v, err := qs.NewEncoder().Values(SaveContentQuery{DeviceID: "dev-1", Optimistic: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Encode(); !strings.Contains(got, "optimistic=true") {
		t.Errorf("expected optimistic=true in query, got %q", got)
	}
}

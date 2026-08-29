package romm

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func TestGetSavesForROMIDsValidatesWholeResponse(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		wantID int
		failed bool
	}{
		{name: "matching result", status: http.StatusOK, body: `[{"id":1,"rom_id":7}]`, wantID: 1},
		{name: "empty", status: http.StatusOK, body: `[]`},
		{name: "no content", status: http.StatusNoContent},
		{name: "partial content", status: http.StatusPartialContent, body: `[]`, failed: true},
		{name: "malformed", status: http.StatusOK, body: `{}`, failed: true},
		{name: "truncated", status: http.StatusOK, body: `[{"id":1,"rom_id":7}`, failed: true},
		{name: "trailing data", status: http.StatusOK, body: `[{"id":1,"rom_id":7}] true`, failed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if query := r.URL.Query(); query.Get("device_id") != "device-1" || query.Get("rom_ids") != "7" {
					t.Errorf("query = %q", r.URL.RawQuery)
				}
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			saves, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7})
			if (err != nil) != tt.failed {
				t.Fatalf("saves=%+v err=%v", saves, err)
			}
			if tt.failed && len(saves) != 0 {
				t.Fatalf("invalid response leaked saves: %+v", saves)
			}
			if tt.wantID != 0 && (len(saves) != 1 || saves[0].ID != tt.wantID) {
				t.Fatalf("saves=%+v, want ID %d", saves, tt.wantID)
			}
		})
	}
}

func TestGetSavesForROMIDsBatchesAndAppliesLimitsPerResponse(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		ids := r.URL.Query()["rom_ids"]
		wantIDs := 500
		if requests == 2 {
			wantIDs = 1
		}
		if len(ids) != wantIDs {
			t.Errorf("rom_ids count = %d, want %d", len(ids), wantIDs)
			return
		}
		if requests == 1 && (ids[0] != "1" || ids[len(ids)-1] != "500") {
			t.Errorf("first batch bounds = %q..%q", ids[0], ids[len(ids)-1])
		}
		if requests == 2 && ids[0] != "501" {
			t.Errorf("second batch = %q", ids)
		}
		romID := 1
		if requests == 2 {
			romID = 501
		}
		_ = json.NewEncoder(w).Encode([]Save{{ID: requests*10 + 1, RomID: romID}, {ID: requests*10 + 2, RomID: romID}})
	}))
	defer server.Close()

	romIDs := []int{0, -1, 1, 501}
	for romID := 1; romID <= 501; romID++ {
		romIDs = append(romIDs, romID)
	}
	saves, err := NewClient(server.URL).getSavesForROMIDs(
		SaveQuery{DeviceID: "device-1"},
		romIDs,
		saveListLimits{bytes: maxSaveListBytes, records: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(saves) != 4 {
		t.Fatalf("requests=%d saves=%+v", requests, saves)
	}
}

func TestGetSavesForROMIDsRefetchesBroadWhenServerIgnoresScope(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Has("rom_ids") {
			_ = json.NewEncoder(w).Encode([]Save{{ID: 1, RomID: 1}, {ID: 501, RomID: 501}})
			return
		}
		_ = json.NewEncoder(w).Encode([]Save{{ID: 1, RomID: 1}, {ID: 501, RomID: 501}, {ID: 1001, RomID: 1001}, {ID: 9999, RomID: 9999}})
	}))
	defer server.Close()

	romIDs := make([]int, 1001)
	for i := range romIDs {
		romIDs[i] = i + 1
	}
	saves, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, romIDs)
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || len(saves) != 3 {
		t.Fatalf("requests=%d saves=%+v", requests, saves)
	}
}

func TestGetSavesForROMIDsLaterBatchFailureLeaksNothing(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 2 {
			http.Error(w, "failed", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode([]Save{{ID: 1, RomID: 1}})
	}))
	defer server.Close()

	romIDs := make([]int, 501)
	for i := range romIDs {
		romIDs[i] = i + 1
	}
	saves, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, romIDs)
	if err == nil || len(saves) != 0 {
		t.Fatalf("requests=%d saves=%+v err=%v", requests, saves, err)
	}
}

func TestDecodeSaveListLimitsAndInterruptionLeakNothing(t *testing.T) {
	wanted := map[int]struct{}{7: {}}
	tests := []struct {
		name   string
		reader io.Reader
		limits saveListLimits
	}{
		{
			name:   "10001 records",
			reader: strings.NewReader("[" + strings.Repeat(`{"rom_id":7},`, maxSaveListRecords) + `{"rom_id":7}]`),
			limits: saveListLimits{bytes: maxSaveListBytes, records: maxSaveListRecords},
		},
		{
			name:   "over 16 MiB decoded",
			reader: strings.NewReader(`[{"rom_id":7,"file_name":"` + strings.Repeat("x", int(maxSaveListBytes)) + `"}]`),
			limits: saveListLimits{bytes: maxSaveListBytes, records: maxSaveListRecords},
		},
		{
			name: "interrupted",
			reader: io.MultiReader(
				strings.NewReader(`[{"id":1,"rom_id":7},`),
				errorReader{err: errors.New("connection lost")},
			),
			limits: saveListLimits{bytes: maxSaveListBytes, records: maxSaveListRecords},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saves, err := decodeSaveList(tt.reader, wanted, nil, tt.limits)
			if err == nil || len(saves) != 0 {
				t.Fatalf("invalid response returned saves=%+v err=%v", saves, err)
			}
		})
	}
}

func TestGetSavesForROMIDsLimitsDecodedGzipBody(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, _ = fmt.Fprintf(zw, `[{"rom_id":7,"file_name":"%s"}]`, strings.Repeat("x", int(maxSaveListBytes)))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(compressed.Bytes())
	}))
	defer server.Close()

	saves, err := NewClient(server.URL).GetSavesForROMIDs(SaveQuery{DeviceID: "device-1"}, []int{7})
	if err == nil || len(saves) != 0 {
		t.Fatalf("compressed oversized response returned saves=%+v err=%v", saves, err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func BenchmarkDecodeSaveList10000(b *testing.B) {
	payload := "[" + strings.Repeat(`{"rom_id":7},`, maxSaveListRecords-1) + `{"rom_id":7}]`
	wanted := map[int]struct{}{7: {}}
	limits := saveListLimits{bytes: maxSaveListBytes, records: maxSaveListRecords}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := decodeSaveList(strings.NewReader(payload), wanted, nil, limits); err != nil {
			b.Fatal(err)
		}
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

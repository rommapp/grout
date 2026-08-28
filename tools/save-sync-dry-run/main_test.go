package main

import (
	"bytes"
	"errors"
	"fmt"
	"grout/romm"
	groutsync "grout/sync"
	"strings"
	"testing"
)

const validMeasurement = "scan_saves_ms=1 load_recorded_state_ms=2 build_client_save_states_ms=3 negotiate_ms=4 scan_roms_ms=5 resolve_local_roms_ms=6 map_operations_ms=7 discover_remote_only_saves_ms=8 total_ms=9 local_saves=10 installed_roms=11 resolved_roms=12 uncovered_roms=13 remote_discovery_requests=14 platforms_queried=15 fallback_per_rom_requests=16 remote_save_records=17 resulting_sync_items=4"

const validHardenedMeasurement = "scan_saves_ms=1 load_recorded_state_ms=2 build_client_save_states_ms=3 negotiate_ms=4 scan_roms_ms=5 resolve_local_roms_ms=6 map_operations_ms=7 discover_remote_only_saves_ms=8 total_ms=9 local_saves=10 installed_roms=11 resolved_roms=12 uncovered_roms=13 remote_discovery_requests=14 platforms_queried=15 fallback_per_rom_requests=16 bulk_records_seen=18 bulk_bytes_read=19 filtered_records=20 fallback_reason=none remote_save_records=17 resulting_sync_items=4"

func measurementLog(metrics string) []byte {
	return []byte(fmt.Sprintf("{\"level\":\"DEBUG\",\"msg\":\"Save sync baseline measurement\",\"metrics\":%q}\n", metrics))
}

func TestHardenedMeasurementRecordParsesAndCaptures(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		if _, err := parseMeasurement(validHardenedMeasurement); err != nil {
			t.Fatalf("parse hardened measurement: %v", err)
		}
	})
	t.Run("capture", func(t *testing.T) {
		got, err := captureBaselineMeasurement(measurementLog(validHardenedMeasurement))
		if err != nil {
			t.Fatalf("capture hardened measurement: %v", err)
		}
		if got != validHardenedMeasurement {
			t.Fatalf("captured measurement = %q", got)
		}
	})
}

func TestMeasurementSchemasAcceptExactLegacyAndHardenedRecords(t *testing.T) {
	tests := []struct {
		name    string
		metrics string
		schema  measurementSchema
		ints    int
		strings int
	}{
		{name: "legacy", metrics: validMeasurement, schema: legacyMeasurementSchema, ints: 18},
		{name: "hardened", metrics: validHardenedMeasurement, schema: hardenedMeasurementSchema, ints: 21, strings: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseMeasurement(tt.metrics)
			if err != nil {
				t.Fatalf("parseMeasurement: %v", err)
			}
			if parsed.schema != tt.schema || len(parsed.integers) != tt.ints || len(parsed.strings) != tt.strings {
				t.Fatalf("parsed = schema %q, %d ints, %d strings", parsed.schema, len(parsed.integers), len(parsed.strings))
			}
			fields := strings.Fields(tt.metrics)
			for left, right := 0, len(fields)-1; left < right; left, right = left+1, right-1 {
				fields[left], fields[right] = fields[right], fields[left]
			}
			if _, err := parseMeasurement(strings.Join(fields, " ")); err != nil {
				t.Fatalf("order-independent parse: %v", err)
			}
		})
	}
}

func TestHardenedMeasurementAcceptsEveryFallbackReason(t *testing.T) {
	reasons := []string{"none", "empty_device", "transport", "http_status", "invalid_query", "decode", "byte_limit", "record_limit", "trailing_data", "unknown"}
	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			metrics := strings.Replace(validHardenedMeasurement, "fallback_reason=none", "fallback_reason="+reason, 1)
			parsed, err := parseMeasurement(metrics)
			if err != nil {
				t.Fatalf("parseMeasurement: %v", err)
			}
			if parsed.strings["fallback_reason"] != reason {
				t.Fatalf("fallback_reason = %q, want %q", parsed.strings["fallback_reason"], reason)
			}
		})
	}
}

func TestMeasurementRejectsUnsupportedFieldCounts(t *testing.T) {
	legacyFields := strings.Fields(validMeasurement)
	hardenedFields := strings.Fields(validHardenedMeasurement)
	for _, count := range []int{17, 19, 20, 21, 23} {
		t.Run(fmt.Sprintf("fields_%d", count), func(t *testing.T) {
			fields := hardenedFields
			if count == 17 {
				fields = legacyFields
			}
			candidate := append([]string(nil), fields...)
			for len(candidate) < count {
				candidate = append(candidate, fmt.Sprintf("extra_%d=0", len(candidate)))
			}
			candidate = candidate[:count]
			if _, err := parseMeasurement(strings.Join(candidate, " ")); err == nil || err.Error() != "measurement_field_count" {
				t.Fatalf("error = %v, want measurement_field_count", err)
			}
		})
	}
}

func TestMeasurementRejectsUnknownDuplicateAndMissingKeys(t *testing.T) {
	tests := []struct {
		name    string
		metrics string
		wantErr string
	}{
		{name: "unknown", metrics: strings.Replace(validHardenedMeasurement, "bulk_records_seen=18", "raw_secret=18", 1), wantErr: "measurement_key_invalid"},
		{name: "duplicate", metrics: strings.Replace(validHardenedMeasurement, "bulk_records_seen=18", "bulk_bytes_read=18", 1), wantErr: "measurement_key_duplicate"},
		{name: "missing", metrics: strings.Join(strings.Fields(validHardenedMeasurement)[:21], " "), wantErr: "measurement_field_count"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseMeasurement(tt.metrics); err == nil || err.Error() != tt.wantErr {
				t.Fatalf("error = %v, want %s", err, tt.wantErr)
			}
		})
	}
}

func TestHardenedMeasurementRejectsInvalidNewIntegers(t *testing.T) {
	invalid := []string{"-1", "+1", "9223372036854775808", "", "1.2", "abc"}
	for _, key := range []string{"bulk_records_seen", "bulk_bytes_read", "filtered_records"} {
		for _, value := range invalid {
			t.Run(key+"_"+value, func(t *testing.T) {
				metrics := strings.Replace(validHardenedMeasurement, key+"="+map[string]string{"bulk_records_seen": "18", "bulk_bytes_read": "19", "filtered_records": "20"}[key], key+"="+value, 1)
				if _, err := parseMeasurement(metrics); err == nil {
					t.Fatal("parseMeasurement unexpectedly succeeded")
				}
			})
		}
	}
}

func TestHardenedMeasurementRejectsInvalidFallbackReasons(t *testing.T) {
	for _, reason := range []string{"", "NONE", "timeout", "none=extra", "raw/path"} {
		t.Run(reason, func(t *testing.T) {
			metrics := strings.Replace(validHardenedMeasurement, "fallback_reason=none", "fallback_reason="+reason, 1)
			if _, err := parseMeasurement(metrics); err == nil {
				t.Fatal("parseMeasurement unexpectedly succeeded")
			}
		})
	}
}

func TestMalformedMatchingMeasurementFailsClosed(t *testing.T) {
	logs := [][]byte{
		measurementLog(strings.Replace(validHardenedMeasurement, "fallback_reason=none", "fallback_reason=raw-secret", 1)),
		[]byte(`{"msg":"Save sync baseline measurement","metrics":`),
		measurementLog(strings.Replace(validHardenedMeasurement, "bulk_bytes_read=19", "bulk_bytes_read=-19", 1)),
	}
	for index, raw := range logs {
		t.Run(fmt.Sprintf("record_%d", index), func(t *testing.T) {
			if got, err := captureBaselineMeasurement(raw); err == nil || got != "" {
				t.Fatalf("capture = %q, %v; want fail closed", got, err)
			}
		})
	}
}

func TestHardenedAggregateOutputIsTypedAndSanitized(t *testing.T) {
	result := groutsync.SyncResult{SessionID: 93, Items: []groutsync.SyncItem{
		{Action: groutsync.ActionUpload}, {Action: groutsync.ActionDownload}, {Action: groutsync.ActionConflict}, {Action: groutsync.ActionSkip},
	}}
	outcome := resolveAndClose(
		func() (groutsync.SyncResult, error) { return result, nil },
		func(_ int, payload romm.SyncCompletePayload) error {
			if payload.OperationsCompleted != 0 || payload.OperationsFailed != 0 {
				t.Fatalf("nonzero completion payload: %+v", payload)
			}
			return nil
		},
	)
	raw := append([]byte("token=secret path=/private/save\n"), measurementLog(validHardenedMeasurement)...)
	var stdout, stderr bytes.Buffer
	if code := writeDiagnostic(&stdout, &stderr, outcome, raw); code != 0 {
		t.Fatalf("writeDiagnostic code = %d, stderr = %q", code, stderr.String())
	}
	want := "diagnostic=grout-resolve-only\n" +
		"session_closure=completed\noperations_completed=0\noperations_failed=0\n" +
		"sync_items_total=4\nsync_items_upload=1\nsync_items_download=1\nsync_items_conflict=1\nsync_items_skip=1\n" +
		"measurement_scan_saves_ms=1\nmeasurement_load_recorded_state_ms=2\nmeasurement_build_client_save_states_ms=3\n" +
		"measurement_negotiate_ms=4\nmeasurement_scan_roms_ms=5\nmeasurement_resolve_local_roms_ms=6\n" +
		"measurement_map_operations_ms=7\nmeasurement_discover_remote_only_saves_ms=8\nmeasurement_total_ms=9\n" +
		"measurement_local_saves=10\nmeasurement_installed_roms=11\nmeasurement_resolved_roms=12\n" +
		"measurement_uncovered_roms=13\nmeasurement_remote_discovery_requests=14\nmeasurement_platforms_queried=15\n" +
		"measurement_fallback_per_rom_requests=16\nmeasurement_bulk_records_seen=18\nmeasurement_bulk_bytes_read=19\n" +
		"measurement_filtered_records=20\nmeasurement_fallback_reason=none\nmeasurement_remote_save_records=17\n" +
		"measurement_resulting_sync_items=4\n"
	if stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("output mismatch stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, forbidden := range []string{"secret", "/private/save"} {
		if strings.Contains(stdout.String()+stderr.String(), forbidden) {
			t.Fatalf("output leaked %q", forbidden)
		}
	}
}

func TestAggregateOnlyOutputRedactsSensitiveFields(t *testing.T) {
	forbidden := []string{
		"https://romm.private.example", "raw-device-id", "raw-session-id", "Secret ROM",
		"secret.sav", "/storage/saves/secret.sav", "2026-08-27T01:02:03Z",
		"remote-save-777", "secret-token", "secret-content",
	}
	raw := []byte(`{"level":"DEBUG","msg":"raw detail","host":"https://romm.private.example","device":"raw-device-id","session":"raw-session-id","rom":"Secret ROM","filename":"secret.sav","path":"/storage/saves/secret.sav","mtime":"2026-08-27T01:02:03Z","remote_id":"remote-save-777","token":"secret-token","content":"secret-content"}` + "\n")
	raw = append(raw, measurementLog(validMeasurement)...)

	result := groutsync.SyncResult{
		SessionID: 81234,
		Items: []groutsync.SyncItem{
			{Action: groutsync.ActionUpload, LocalSave: groutsync.LocalSave{RomName: forbidden[3], FileName: forbidden[4], FilePath: forbidden[5]}},
			{Action: groutsync.ActionDownload},
			{Action: groutsync.ActionConflict},
			{Action: groutsync.ActionSkip},
		},
	}
	completeCalls := 0
	var completedSession int
	var completedPayload romm.SyncCompletePayload
	outcome := resolveAndClose(
		func() (groutsync.SyncResult, error) { return result, nil },
		func(sessionID int, payload romm.SyncCompletePayload) error {
			completeCalls++
			completedSession = sessionID
			completedPayload = payload
			return nil
		},
	)

	var stdout, stderr bytes.Buffer
	if code := writeDiagnostic(&stdout, &stderr, outcome, raw); code != 0 {
		t.Fatalf("writeDiagnostic code = %d, stderr = %q", code, stderr.String())
	}
	if completeCalls != 1 || completedSession != result.SessionID {
		t.Fatalf("CompleteSession calls/session = %d/%d, want 1/%d", completeCalls, completedSession, result.SessionID)
	}
	if completedPayload.OperationsCompleted != 0 || completedPayload.OperationsFailed != 0 {
		t.Fatalf("completion payload = %+v, want exact zero-op closure", completedPayload)
	}

	want := "diagnostic=grout-resolve-only\n" +
		"session_closure=completed\n" +
		"operations_completed=0\noperations_failed=0\n" +
		"sync_items_total=4\nsync_items_upload=1\nsync_items_download=1\nsync_items_conflict=1\nsync_items_skip=1\n" +
		"measurement_scan_saves_ms=1\nmeasurement_load_recorded_state_ms=2\nmeasurement_build_client_save_states_ms=3\n" +
		"measurement_negotiate_ms=4\nmeasurement_scan_roms_ms=5\nmeasurement_resolve_local_roms_ms=6\n" +
		"measurement_map_operations_ms=7\nmeasurement_discover_remote_only_saves_ms=8\nmeasurement_total_ms=9\n" +
		"measurement_local_saves=10\nmeasurement_installed_roms=11\nmeasurement_resolved_roms=12\n" +
		"measurement_uncovered_roms=13\nmeasurement_remote_discovery_requests=14\nmeasurement_platforms_queried=15\n" +
		"measurement_fallback_per_rom_requests=16\nmeasurement_remote_save_records=17\nmeasurement_resulting_sync_items=4\n"
	if stdout.String() != want {
		t.Fatalf("stdout mismatch\n got: %q\nwant: %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	combined := stdout.String() + stderr.String()
	for _, secret := range forbidden {
		if strings.Contains(combined, secret) {
			t.Errorf("diagnostic output leaked forbidden value %q", secret)
		}
	}
}

func TestDebugFilterEmitsOnlyBaselineMeasurement(t *testing.T) {
	raw := []byte("before\n")
	raw = append(raw, measurementLog(validMeasurement)...)
	raw = append(raw, []byte(`{"level":"DEBUG","msg":"raw detail","token":"secret","path":"/private/save"}`+"\n")...)
	raw = append(raw, measurementLog("total_ms=10 resulting_sync_items=2")...)

	got, err := captureBaselineMeasurement(raw)
	if err != nil {
		t.Fatalf("captureBaselineMeasurement: %v", err)
	}
	if got != validMeasurement {
		t.Fatalf("measurement = %q, want %q", got, validMeasurement)
	}
	for _, forbidden := range []string{"before", "raw detail", "secret", "/private/save", "total_ms=10"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("filtered measurement leaked %q: %q", forbidden, got)
		}
	}
}

func TestResolveOnlyCompletesSessionWithZeroOperations(t *testing.T) {
	calls := 0
	outcome := resolveAndClose(
		func() (groutsync.SyncResult, error) {
			return groutsync.SyncResult{SessionID: 37, Items: []groutsync.SyncItem{{Action: groutsync.ActionUpload}}}, nil
		},
		func(sessionID int, payload romm.SyncCompletePayload) error {
			calls++
			if sessionID != 37 {
				t.Fatalf("CompleteSession session = %d, want 37", sessionID)
			}
			if payload.OperationsCompleted != 0 || payload.OperationsFailed != 0 {
				t.Fatalf("completion payload = %+v, want exact zero-op closure", payload)
			}
			return nil
		},
	)
	if calls != 1 {
		t.Fatalf("CompleteSession calls = %d, want exactly 1", calls)
	}
	if outcome.errKind != "" {
		t.Fatalf("resolveAndClose errKind = %q, want success", outcome.errKind)
	}
}

func TestResolveOnlyCompletesSessionOnPostResolveFailure(t *testing.T) {
	tests := []struct {
		name  string
		items []groutsync.SyncItem
		raw   []byte
	}{
		{name: "missing measurement", items: []groutsync.SyncItem{{Action: groutsync.ActionUpload}}},
		{name: "unknown action", items: []groutsync.SyncItem{{Action: groutsync.SyncAction(99)}}, raw: measurementLog(validMeasurement)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			outcome := resolveAndClose(
				func() (groutsync.SyncResult, error) {
					return groutsync.SyncResult{SessionID: 41, Items: tt.items}, nil
				},
				func(_ int, payload romm.SyncCompletePayload) error {
					calls++
					if payload.OperationsCompleted != 0 || payload.OperationsFailed != 0 {
						t.Fatalf("nonzero completion payload: %+v", payload)
					}
					return nil
				},
			)
			var stdout, stderr bytes.Buffer
			if code := writeDiagnostic(&stdout, &stderr, outcome, tt.raw); code == 0 {
				t.Fatalf("writeDiagnostic unexpectedly succeeded: %q", stdout.String())
			}
			if calls != 1 {
				t.Fatalf("CompleteSession calls = %d, want exactly 1 before presentation", calls)
			}
			if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "diagnostic_error=") {
				t.Fatalf("unsafe failure output stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestNegotiation_CatchesDuplicateCompletionAfterCloseError(t *testing.T) {
	calls := 0
	outcome := resolveAndClose(
		func() (groutsync.SyncResult, error) { return groutsync.SyncResult{SessionID: 52}, nil },
		func(_ int, payload romm.SyncCompletePayload) error {
			calls++
			if payload.OperationsCompleted != 0 || payload.OperationsFailed != 0 {
				t.Fatalf("nonzero completion payload: %+v", payload)
			}
			return errors.New("host=https://private token=secret path=/private/save")
		},
	)
	var stdout, stderr bytes.Buffer
	if code := writeDiagnostic(&stdout, &stderr, outcome, measurementLog(validMeasurement)); code == 0 {
		t.Fatal("writeDiagnostic unexpectedly succeeded after closure failure")
	}
	if calls != 1 {
		t.Fatalf("CompleteSession calls = %d, want exactly 1 with no retry", calls)
	}
	if stdout.Len() != 0 || stderr.String() != "diagnostic_error=session_close_failed\n" {
		t.Fatalf("unsafe close failure output stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDiagnosticErrors_CatchWrappedRawErrorLeak(t *testing.T) {
	outcome := resolveAndClose(
		func() (groutsync.SyncResult, error) {
			return groutsync.SyncResult{}, errors.New("host=https://private device=raw-id token=secret path=/private/save")
		},
		func(_ int, _ romm.SyncCompletePayload) error { t.Fatal("unexpected CompleteSession call"); return nil },
	)
	var stdout, stderr bytes.Buffer
	if code := writeDiagnostic(&stdout, &stderr, outcome, nil); code == 0 {
		t.Fatal("writeDiagnostic unexpectedly succeeded")
	}
	if stdout.Len() != 0 || stderr.String() != "diagnostic_error=resolve_failed\n" {
		t.Fatalf("raw error escaped: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

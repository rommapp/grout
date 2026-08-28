package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"

	"grout/cache"
	"grout/internal"
	"grout/romm"
	"grout/sync"
)

func main() {
	scenario := flag.String("scenario", "", "run an offline fix-verification scenario instead of a live dry-run "+
		"(slot-switch, nextui-keep, nextui-retroarch, all; requires -tags dryrun)")
	flag.Parse()

	// Offline scenario mode uses synthetic inputs and never connects to a server.
	if *scenario != "" {
		if err := runScenario(*scenario); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	safeStdout := os.Stdout
	outcome, rawLogs, err := runCapturedDiagnostic()
	if err != nil {
		fmt.Fprintln(os.Stderr, "diagnostic_error=log_capture_failed")
		os.Exit(1)
	}
	os.Exit(writeDiagnostic(safeStdout, os.Stderr, outcome, rawLogs))
}

type diagnosticOutcome struct {
	result        sync.SyncResult
	errKind       string
	sessionClosed bool
}

type resolveFunc func() (sync.SyncResult, error)
type completeFunc func(int, romm.SyncCompletePayload) error

func runCapturedDiagnostic() (diagnosticOutcome, []byte, error) {
	originalStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		return diagnosticOutcome{}, nil, err
	}
	os.Stdout = writer

	var captured bytes.Buffer
	copyDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(&captured, reader)
		copyDone <- copyErr
	}()

	// Gabagool always mirrors application logs to stdout and a file. Point the
	// file side at the platform null device and capture stdout in memory so raw
	// DEBUG records never reach a terminal or persistent log.
	gaba.SetLogPath(os.DevNull)
	gaba.SetLogLevel(slog.LevelDebug)
	outcome := runLiveResolveOnly()

	closeErr := writer.Close()
	os.Stdout = originalStdout
	copyErr := <-copyDone
	readCloseErr := reader.Close()
	if closeErr != nil {
		return outcome, nil, closeErr
	}
	if copyErr != nil {
		return outcome, nil, copyErr
	}
	if readCloseErr != nil {
		return outcome, nil, readCloseErr
	}
	return outcome, captured.Bytes(), nil
}

func runLiveResolveOnly() diagnosticOutcome {
	config, err := internal.LoadConfig()
	if err != nil {
		return diagnosticOutcome{errKind: "config_load_failed"}
	}
	if len(config.Hosts) == 0 {
		return diagnosticOutcome{errKind: "host_missing"}
	}

	host := config.Hosts[0]
	if host.DeviceID == "" {
		return diagnosticOutcome{errKind: "device_unregistered"}
	}
	if err := cache.InitCacheManager(host, config); err != nil {
		return diagnosticOutcome{errKind: "cache_init_failed"}
	}
	defer cache.GetCacheManager().Close()

	client := romm.NewClientFromHost(host, config.ApiTimeout.Duration())
	return resolveAndClose(
		func() (sync.SyncResult, error) {
			return sync.ResolveSaveSync(client, config, host.DeviceID)
		},
		client.CompleteSession,
	)
}

func resolveAndClose(resolve resolveFunc, complete completeFunc) diagnosticOutcome {
	result, err := resolve()
	if err != nil {
		return diagnosticOutcome{errKind: "resolve_failed"}
	}
	outcome := diagnosticOutcome{result: result}
	if result.SessionID <= 0 {
		outcome.errKind = "session_missing"
		return outcome
	}
	if err := complete(result.SessionID, romm.SyncCompletePayload{
		OperationsCompleted: 0,
		OperationsFailed:    0,
	}); err != nil {
		outcome.errKind = "session_close_failed"
		return outcome
	}
	outcome.sessionClosed = true
	return outcome
}

type measurementSchema string

const (
	legacyMeasurementSchema   measurementSchema = "legacy-18"
	hardenedMeasurementSchema measurementSchema = "hardened-22"
)

type parsedMeasurement struct {
	schema   measurementSchema
	integers map[string]int64
	strings  map[string]string
}

var legacyMeasurementKeys = []string{
	"scan_saves_ms",
	"load_recorded_state_ms",
	"build_client_save_states_ms",
	"negotiate_ms",
	"scan_roms_ms",
	"resolve_local_roms_ms",
	"map_operations_ms",
	"discover_remote_only_saves_ms",
	"total_ms",
	"local_saves",
	"installed_roms",
	"resolved_roms",
	"uncovered_roms",
	"remote_discovery_requests",
	"platforms_queried",
	"fallback_per_rom_requests",
	"remote_save_records",
	"resulting_sync_items",
}

var hardenedMeasurementKeys = []string{
	"scan_saves_ms",
	"load_recorded_state_ms",
	"build_client_save_states_ms",
	"negotiate_ms",
	"scan_roms_ms",
	"resolve_local_roms_ms",
	"map_operations_ms",
	"discover_remote_only_saves_ms",
	"total_ms",
	"local_saves",
	"installed_roms",
	"resolved_roms",
	"uncovered_roms",
	"remote_discovery_requests",
	"platforms_queried",
	"fallback_per_rom_requests",
	"bulk_records_seen",
	"bulk_bytes_read",
	"filtered_records",
	"fallback_reason",
	"remote_save_records",
	"resulting_sync_items",
}

var fallbackReasons = map[string]struct{}{
	"none": {}, "empty_device": {}, "transport": {}, "http_status": {},
	"invalid_query": {}, "decode": {}, "byte_limit": {}, "record_limit": {},
	"trailing_data": {}, "unknown": {},
}

func captureBaselineMeasurement(raw []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var record struct {
			Msg     string
			Metrics string
		}
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue
		}
		if record.Msg != "Save sync baseline measurement" {
			continue
		}
		if _, err := parseMeasurement(record.Metrics); err != nil {
			continue
		}
		return record.Metrics, nil
	}
	if err := scanner.Err(); err != nil {
		return "", errors.New("measurement_scan_failed")
	}
	return "", errors.New("measurement_missing")
}

func parseMeasurement(metrics string) (parsedMeasurement, error) {
	fields := strings.Fields(metrics)
	var schema measurementSchema
	var keys []string
	switch len(fields) {
	case len(legacyMeasurementKeys):
		schema, keys = legacyMeasurementSchema, legacyMeasurementKeys
	case len(hardenedMeasurementKeys):
		schema, keys = hardenedMeasurementSchema, hardenedMeasurementKeys
	default:
		return parsedMeasurement{}, errors.New("measurement_field_count")
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}
	parsed := parsedMeasurement{
		schema:   schema,
		integers: make(map[string]int64, len(keys)),
		strings:  make(map[string]string, 1),
	}
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 {
			return parsedMeasurement{}, errors.New("measurement_field_invalid")
		}
		if _, ok := allowed[parts[0]]; !ok {
			return parsedMeasurement{}, errors.New("measurement_key_invalid")
		}
		if _, duplicate := parsed.integers[parts[0]]; duplicate {
			return parsedMeasurement{}, errors.New("measurement_key_duplicate")
		}
		if _, duplicate := parsed.strings[parts[0]]; duplicate {
			return parsedMeasurement{}, errors.New("measurement_key_duplicate")
		}
		if parts[0] == "fallback_reason" {
			if _, ok := fallbackReasons[parts[1]]; !ok {
				return parsedMeasurement{}, errors.New("measurement_value_invalid")
			}
			parsed.strings[parts[0]] = parts[1]
			continue
		}
		if parts[1] == "" || strings.IndexFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return parsedMeasurement{}, errors.New("measurement_value_invalid")
		}
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || value < 0 {
			return parsedMeasurement{}, errors.New("measurement_value_invalid")
		}
		parsed.integers[parts[0]] = value
	}
	for _, key := range keys {
		_, integerOK := parsed.integers[key]
		_, stringOK := parsed.strings[key]
		if !integerOK && !stringOK {
			return parsedMeasurement{}, errors.New("measurement_key_missing")
		}
	}
	return parsed, nil
}

func writeDiagnostic(stdout, stderr io.Writer, outcome diagnosticOutcome, rawLogs []byte) int {
	if outcome.errKind != "" {
		fmt.Fprintf(stderr, "diagnostic_error=%s\n", outcome.errKind)
		return 1
	}
	if !outcome.sessionClosed {
		fmt.Fprintln(stderr, "diagnostic_error=session_not_closed")
		return 1
	}

	measurement, err := captureBaselineMeasurement(rawLogs)
	if err != nil {
		fmt.Fprintln(stderr, "diagnostic_error=measurement_missing")
		return 1
	}
	measurementValues, err := parseMeasurement(measurement)
	if err != nil {
		fmt.Fprintln(stderr, "diagnostic_error=measurement_invalid")
		return 1
	}

	counts := map[sync.SyncAction]int{
		sync.ActionUpload:   0,
		sync.ActionDownload: 0,
		sync.ActionConflict: 0,
		sync.ActionSkip:     0,
	}
	for _, item := range outcome.result.Items {
		if _, ok := counts[item.Action]; !ok {
			fmt.Fprintln(stderr, "diagnostic_error=unsupported_action")
			return 1
		}
		counts[item.Action]++
	}
	if measurementValues.integers["resulting_sync_items"] != int64(len(outcome.result.Items)) {
		fmt.Fprintln(stderr, "diagnostic_error=measurement_mismatch")
		return 1
	}

	var output bytes.Buffer
	fmt.Fprintln(&output, "diagnostic=grout-resolve-only")
	fmt.Fprintln(&output, "session_closure=completed")
	fmt.Fprintln(&output, "operations_completed=0")
	fmt.Fprintln(&output, "operations_failed=0")
	fmt.Fprintf(&output, "sync_items_total=%d\n", len(outcome.result.Items))
	fmt.Fprintf(&output, "sync_items_upload=%d\n", counts[sync.ActionUpload])
	fmt.Fprintf(&output, "sync_items_download=%d\n", counts[sync.ActionDownload])
	fmt.Fprintf(&output, "sync_items_conflict=%d\n", counts[sync.ActionConflict])
	fmt.Fprintf(&output, "sync_items_skip=%d\n", counts[sync.ActionSkip])
	keys := legacyMeasurementKeys
	if measurementValues.schema == hardenedMeasurementSchema {
		keys = hardenedMeasurementKeys
	}
	for _, key := range keys {
		if value, ok := measurementValues.integers[key]; ok {
			fmt.Fprintf(&output, "measurement_%s=%d\n", key, value)
			continue
		}
		fmt.Fprintf(&output, "measurement_%s=%s\n", key, measurementValues.strings[key])
	}
	if _, err := io.Copy(stdout, &output); err != nil {
		fmt.Fprintln(stderr, "diagnostic_error=output_failed")
		return 1
	}
	return 0
}

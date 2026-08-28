package sync

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"grout/romm"
)

const sacrificialDeviceID = "grout-correctness-matrix"

func TestSacrificialUploadSendsExactBytesAndPreservesLocal(t *testing.T) {
	root := t.TempDir()
	savePath := filepath.Join(root, "Upload Game.srm")
	want := []byte("sacrificial-local-upload-v1")
	if err := os.WriteFile(savePath, want, 0o600); err != nil {
		t.Fatal(err)
	}

	uploaded := make(chan []byte, 1)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/api/saves" {
			t.Errorf("request = %s %s, want POST /api/saves", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		wantQuery := map[string]string{
			"autocleanup":       "true",
			"autocleanup_limit": "10",
			"device_id":         sacrificialDeviceID,
			"emulator":          filepath.Base(root),
			"rom_id":            "7001",
			"slot":              "autosave",
		}
		query := r.URL.Query()
		if len(query) != len(wantQuery) {
			t.Errorf("query = %v, want %v", query, wantQuery)
		}
		for key, value := range wantQuery {
			if query.Get(key) != value {
				t.Errorf("query[%q] = %q, want %q", key, query.Get(key), value)
			}
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			http.Error(w, "bad multipart", http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("saveFile")
		if err != nil {
			t.Errorf("saveFile: %v", err)
			http.Error(w, "missing saveFile", http.StatusBadRequest)
			return
		}
		defer file.Close()
		body, err := io.ReadAll(file)
		if err != nil {
			t.Errorf("read saveFile: %v", err)
			http.Error(w, "read failed", http.StatusInternalServerError)
			return
		}
		uploaded <- body
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(romm.Save{
			ID:        7101,
			RomID:     7001,
			FileName:  filepath.Base(savePath),
			UpdatedAt: time.Now().UTC().Truncate(time.Second),
		})
	}))
	t.Cleanup(server.Close)

	item := SyncItem{
		LocalSave: LocalSave{
			RomID:       7001,
			RomName:     "Sacrificial Upload Game",
			FileName:    filepath.Base(savePath),
			FilePath:    savePath,
			EmulatorDir: root,
		},
		Action:     ActionUpload,
		TargetSlot: "autosave",
	}
	report := ExecuteActions(romm.NewClient(server.URL), nil, sacrificialDeviceID, []SyncItem{item}, nil)
	if report.Uploaded != 1 || report.Errors != 0 || !report.Items[0].Success {
		t.Fatalf("report = %+v", report)
	}
	if requests.Load() != 1 {
		t.Fatalf("requests = %d, want 1", requests.Load())
	}
	if got := <-uploaded; !bytes.Equal(got, want) {
		t.Fatalf("uploaded = %q, want %q", got, want)
	}
	got, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("local changed to %q", got)
	}
}

func TestSacrificialDownloadsPreserveOrBackupLiveSave(t *testing.T) {
	tests := []struct {
		name       string
		initial    []byte
		remote     []byte
		status     int
		offline    bool
		wantPassed bool
	}{
		{name: "remote_only", remote: []byte("remote-only-v1"), status: http.StatusOK, wantPassed: true},
		{name: "overwrite", initial: []byte("local-v1"), remote: []byte("remote-v2"), status: http.StatusOK, wantPassed: true},
		{name: "failed_fetch", initial: []byte("local-v1"), status: http.StatusBadGateway},
		{name: "offline", initial: []byte("local-v1"), offline: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			savePath := filepath.Join(root, "Download Game.srm")
			if tc.initial != nil {
				if err := os.WriteFile(savePath, tc.initial, 0o600); err != nil {
					t.Fatal(err)
				}
			}

			var confirms atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/api/saves/7201/content":
					if r.URL.Query().Get("device_id") != sacrificialDeviceID {
						t.Errorf("device_id = %q", r.URL.Query().Get("device_id"))
					}
					if tc.status != http.StatusOK {
						http.Error(w, "sacrificial failure", tc.status)
						return
					}
					_, _ = w.Write(tc.remote)
				case r.Method == http.MethodPost && r.URL.Path == "/api/saves/7201/downloaded":
					confirms.Add(1)
					var body romm.SaveDeviceBody
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("confirmation body: %v", err)
					}
					if body.DeviceID != sacrificialDeviceID {
						t.Errorf("confirmation device_id = %q", body.DeviceID)
					}
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			if tc.offline {
				server.Close()
			}

			item := SyncItem{
				LocalSave: LocalSave{
					RomID:    7002,
					RomName:  "Sacrificial Download Game",
					FSSlug:   "gba",
					FileName: filepath.Base(savePath),
					FilePath: savePath,
				},
				RemoteSave: &romm.Save{
					ID:            7201,
					RomID:         7002,
					FileName:      filepath.Base(savePath),
					FileExtension: "srm",
					UpdatedAt:     time.Now().UTC().Truncate(time.Second),
				},
				Action: ActionDownload,
			}
			report := ExecuteActions(romm.NewClient(server.URL), nil, sacrificialDeviceID, []SyncItem{item}, nil)
			if tc.wantPassed {
				if report.Downloaded != 1 || report.Errors != 0 || !report.Items[0].Success {
					t.Fatalf("report = %+v", report)
				}
			} else if report.Downloaded != 0 || report.Errors != 1 || report.Items[0].Success {
				t.Fatalf("report = %+v", report)
			}

			wantLive := tc.initial
			if tc.wantPassed {
				wantLive = tc.remote
			}
			gotLive, err := os.ReadFile(savePath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotLive, wantLive) {
				t.Fatalf("live = %q, want %q", gotLive, wantLive)
			}
			if tc.initial == nil {
				if _, err := os.Stat(filepath.Join(root, ".backup")); !os.IsNotExist(err) {
					t.Fatalf("remote-only backup stat error = %v", err)
				}
			} else {
				entries, err := os.ReadDir(filepath.Join(root, ".backup"))
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 1 {
					t.Fatalf("backup count = %d, want 1", len(entries))
				}
				gotBackup, err := os.ReadFile(filepath.Join(root, ".backup", entries[0].Name()))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotBackup, tc.initial) {
					t.Fatalf("backup = %q, want %q", gotBackup, tc.initial)
				}
			}
			wantConfirms := int32(0)
			if tc.wantPassed {
				wantConfirms = 1
			}
			if confirms.Load() != wantConfirms {
				t.Fatalf("confirmations = %d, want %d", confirms.Load(), wantConfirms)
			}
		})
	}
}

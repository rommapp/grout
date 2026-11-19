package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"grout/core/config"
)

func runStatus(t *testing.T, dir string) (int, map[string]any) {
	var out, errb bytes.Buffer
	code := Run(context.Background(), Env{Dir: dir, Stdout: &out, Stderr: &errb}, []string{"status"})
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %q stderr=%q", out.String(), errb.String())
	}
	return code, got
}

func hostConfig(t *testing.T, url string) string {
	dir := t.TempDir()
	config.Save(dir, &config.Config{Hosts: []config.Host{{RootURI: url, Token: "t"}}})
	return dir
}

func TestStatusReady(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"SYSTEM":{"VERSION":"5.5.0"},"SAVE_SYNC":{"SNAPSHOTS":true}}`))
	}))
	defer srv.Close()
	code, got := runStatus(t, hostConfig(t, srv.URL))
	if code != ExitOK || got["ready"] != true || got["snapshots"] != true || got["server_version"] != "5.5.0" || got["config_source"] != "settings" || got["host"] != srv.URL || got["error"] != "" {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestStatusTooOld(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"SYSTEM":{"VERSION":"4.9.2"}}`))
	}))
	defer srv.Close()
	code, got := runStatus(t, hostConfig(t, srv.URL))
	if code != ExitTooOld || got["ready"] != false || got["error"] == "" || got["server_version"] != "4.9.2" {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestStatusUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	code, got := runStatus(t, hostConfig(t, srv.URL))
	if code != ExitServer || got["error"] == "" || got["ready"] != false {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestStatusUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	code, got := runStatus(t, hostConfig(t, url))
	if code != ExitServer || got["error"] == "" {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestStatusNotConfigured(t *testing.T) {
	code, got := runStatus(t, t.TempDir())
	if code != ExitNotConfigured || got["config_source"] != "new" || got["ready"] != false {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestStatusCorruptSettings(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, config.FileName), []byte("{"), 0644)
	code, got := runStatus(t, dir)
	if code != ExitNotConfigured || got["error"] == "" {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestStatusAfterV2Import(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"SYSTEM":{"VERSION":"5.5.0"},"SAVE_SYNC":{"SNAPSHOTS":true}}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"hosts":[{"root_uri":"`+srv.URL+`","token":"t"}]}`), 0644)
	code, got := runStatus(t, dir)
	if code != ExitOK || got["config_source"] != "imported_v2" {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if Run(context.Background(), Env{Dir: t.TempDir(), Stdout: &out, Stderr: &errb}, []string{"nope"}) != ExitUsage || errb.Len() == 0 {
		t.Fatal("usage not reported")
	}
}

func TestNoCommand(t *testing.T) {
	var out, errb bytes.Buffer
	if Run(context.Background(), Env{Dir: t.TempDir(), Stdout: &out, Stderr: &errb}, nil) != ExitUsage || errb.Len() == 0 {
		t.Fatal("usage not reported")
	}
}

func TestStatusAppliesConfiguredLogLevel(t *testing.T) {
	dir := t.TempDir()
	config.Save(dir, &config.Config{LogLevel: config.LogLevelDebug})
	var out, errb bytes.Buffer
	level := new(slog.LevelVar)
	level.Set(slog.LevelError)
	Run(context.Background(), Env{Dir: dir, Stdout: &out, Stderr: &errb, LogLevel: level}, []string{"status"})
	if level.Level() != slog.LevelDebug {
		t.Fatalf("level %v", level.Level())
	}
}

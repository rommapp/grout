package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"grout/core/config"
	"grout/test/mockroot"
)

func libraryServer(t *testing.T, snapshots bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/heartbeat":
			if snapshots {
				w.Write([]byte(`{"SYSTEM":{"VERSION":"5.5.0"},"SAVE_SYNC":{"SNAPSHOTS":true}}`))
			} else {
				w.Write([]byte(`{"SYSTEM":{"VERSION":"4.9.2"}}`))
			}
		case "/api/platforms":
			w.Write([]byte(`[{"id":1,"slug":"gba","fs_slug":"gba","name":"Game Boy Advance","rom_count":2}]`))
		case "/api/platforms/identifiers":
			w.Write([]byte(`[1]`))
		case "/api/roms":
			w.Write([]byte(`{"items":[{"id":1,"platform_id":1,"name":"Tetris","fs_name":"Tetris.gba","fs_name_no_ext":"Tetris","has_simple_single_file":true,"files":[{"id":11,"file_name":"Tetris.gba"}]},{"id":2,"platform_id":1,"name":"Metroid","fs_name":"Metroid.gba","fs_name_no_ext":"Metroid","has_simple_single_file":true,"files":[{"id":21,"file_name":"Metroid.gba"}]}]}`))
		case "/api/roms/identifiers":
			w.Write([]byte(`[1,2]`))
		case "/api/collections", "/api/collections/smart", "/api/collections/virtual":
			w.Write([]byte(`[]`))
		case "/api/collections/identifiers", "/api/collections/smart/identifiers", "/api/collections/virtual/identifiers":
			w.Write([]byte(`[]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runLibrary(t *testing.T, dir string, env map[string]string, args ...string) (int, map[string]any) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(context.Background(), Env{
		Dir:    dir,
		Stdout: &out,
		Stderr: &errb,
		Getenv: func(k string) string { return env[k] },
	}, append([]string{"library"}, args...))
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout not JSON: %q stderr=%q", out.String(), errb.String())
	}
	return code, got
}

func libraryDir(t *testing.T, url string) string {
	t.Helper()
	dir := t.TempDir()
	err := config.Save(dir, &config.Config{
		Hosts:             []config.Host{{RootURI: url, Token: "t"}},
		DirectoryMappings: map[string]config.DirectoryMapping{"gba": {RomMSlug: "gba"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLibraryRefreshEndToEnd(t *testing.T) {
	srv := libraryServer(t, true)
	root := filepath.Join(t.TempDir(), "root")
	env, err := mockroot.Build(root, mockroot.Options{Firmware: "nextui"})
	if err != nil {
		t.Fatal(err)
	}
	rom := filepath.Join(root, "mnt", "SDCARD", "Roms", "Game Boy Advance (GBA)", "Tetris.gba")
	if err := os.WriteFile(rom, []byte("rom"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := libraryDir(t, srv.URL)

	for _, args := range [][]string{{"refresh"}, {"refresh", "--full"}} {
		code, got := runLibrary(t, dir, env, args...)
		refresh, _ := got["refresh"].(map[string]any)
		scan, _ := got["scan"].(map[string]any)
		skipped, ok := scan["skipped_folders"].([]any)
		if code != ExitOK || got["firmware"] != "nextui" || got["error"] != "" || refresh["games"] != float64(2) || scan["matched"] != float64(1) || !ok || len(skipped) != 0 {
			t.Fatalf("args=%v code=%d got=%v", args, code, got)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".cache", "library.db")); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryRefreshUnknownFirmware(t *testing.T) {
	srv := libraryServer(t, true)
	code, got := runLibrary(t, libraryDir(t, srv.URL), map[string]string{}, "refresh")
	if code != ExitNotConfigured || got["error"] == "" {
		t.Fatalf("code=%d got=%v", code, got)
	}
}

func TestLibraryRefreshTooOld(t *testing.T) {
	srv := libraryServer(t, false)
	dir := libraryDir(t, srv.URL)
	code, got := runLibrary(t, dir, map[string]string{"CFW": "NEXTUI"}, "refresh")
	if code != ExitTooOld || got["error"] == "" {
		t.Fatalf("code=%d got=%v", code, got)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cache", "library.db")); !os.IsNotExist(err) {
		t.Fatalf("library.db exists: %v", err)
	}
}

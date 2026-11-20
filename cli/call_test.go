package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/rommapp/grout/api"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
	Error  *api.ErrorBody  `json:"error"`
}

func runCLI(t *testing.T, dir string, args ...string) (ExitCode, envelope, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(context.Background(), Env{
		Dir:      dir,
		Stdout:   &out,
		Stderr:   &errb,
		Firmware: "testfw",
		Environ:  map[string]string{"GROUT_ROOT": t.TempDir()},
	}, args)
	var e envelope
	if err := json.Unmarshal(out.Bytes(), &e); err != nil {
		t.Fatalf("stdout not JSON: %q stderr=%q", out.String(), errb.String())
	}
	return code, e, out.String()
}

func TestExitForEveryCode(t *testing.T) {
	want := map[api.Code]ExitCode{
		api.CodeNotConfigured:     ExitNotConfigured,
		api.CodeBadDefinition:     ExitNotConfigured,
		api.CodeServerUnreachable: ExitServer,
		api.CodeUnauthorized:      ExitUnauthorized,
		api.CodeServerTooOld:      ExitTooOld,
		api.CodeBusy:              ExitBusy,
		api.CodeDataDirLocked:     ExitLocked,
		api.CodeBadRequest:        ExitUsage,
		api.CodeCancelled:         ExitCancelled,
		api.CodeStorage:           ExitServer,
		api.CodeInternal:          ExitServer,
	}
	for _, c := range api.Codes() {
		w, ok := want[c]
		if !ok {
			t.Fatalf("no expected exit code for %q", c)
		}
		if got := exitFor(c); got != w {
			t.Errorf("exitFor(%q) = %d, want %d", c, got, w)
		}
	}
	if exitFor("") != ExitOK {
		t.Error("empty code is not ExitOK")
	}
	if exitFor("nonsense") != ExitServer {
		t.Error("unknown code is not ExitServer")
	}
}

func TestStatusPrintsEnvelope(t *testing.T) {
	code, e, raw := runCLI(t, t.TempDir(), "status")
	if code != ExitNotConfigured || e.OK || e.Error == nil || e.Error.Code != api.CodeNotConfigured {
		t.Fatalf("code=%d env=%+v", code, e)
	}
	if !bytes.Contains([]byte(raw), []byte("\n  \"ok\": false")) {
		t.Fatalf("not indented: %q", raw)
	}
}

func TestLibraryRefreshPrintsEnvelopeWithScan(t *testing.T) {
	srv := fakeromm.New(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv.Platforms = []romm.Platform{{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 2, UpdatedAt: base}}
	srv.Roms = fakeromm.Roms(1, 1, 2, base)
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{}); err != nil {
		t.Fatal(err)
	}
	code, e, _ := runCLI(t, dir, "call", "settings.set_host", fmt.Sprintf(`{"root_uri":%q,"token":"t"}`, srv.URL()))
	if code != ExitOK {
		t.Fatalf("set_host code=%d env=%+v", code, e)
	}
	code, e, raw := runCLI(t, dir, "call", "settings.set_directory_mapping", `{"fs_slug":"gba","slug":"gba","enabled":true}`)
	if code != ExitOK {
		t.Fatalf("mapping code=%d env=%+v", code, e)
	}
	code, e, raw = runCLI(t, dir, "library", "refresh", "--full")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%+v", code, e)
	}
	var res struct {
		Refresh struct {
			Platforms int `json:"platforms"`
		} `json:"refresh"`
		Scan json.RawMessage `json:"scan"`
	}
	if err := json.Unmarshal(e.Result, &res); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	if res.Refresh.Platforms <= 0 || len(res.Scan) == 0 || string(res.Scan) == "null" {
		t.Fatalf("result=%s", e.Result)
	}
}

func TestCallRunsAnyMethod(t *testing.T) {
	code, e, _ := runCLI(t, t.TempDir(), "call", "library.dims")
	var dims []string
	if code != ExitOK || !e.OK || json.Unmarshal(e.Result, &dims) != nil || len(dims) != 8 {
		t.Fatalf("code=%d env=%+v dims=%v", code, e, dims)
	}
}

func TestCallBadJSONIsUsage(t *testing.T) {
	code, e, _ := runCLI(t, t.TempDir(), "call", "library.platforms", "{")
	if code != ExitUsage || e.Error == nil || e.Error.Code != api.CodeBadRequest {
		t.Fatalf("code=%d env=%+v", code, e)
	}
}

func TestLockedDataDirExitCode(t *testing.T) {
	dir := t.TempDir()
	s, err := api.Open(context.Background(), api.OpenOptions{
		DataDir:  dir,
		Firmware: api.FirmwareSpec{ID: "testfw"},
		Env:      map[string]string{"GROUT_ROOT": t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	code, e, _ := runCLI(t, dir, "status")
	if code != ExitLocked || e.Error == nil || e.Error.Code != api.CodeDataDirLocked {
		t.Fatalf("code=%d env=%+v", code, e)
	}
}

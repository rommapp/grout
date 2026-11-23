package cli

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/rommapp/grout/api"
	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"

	"github.com/spf13/cobra"
)

func runFW(t *testing.T, dir, fw string, args ...string) (ExitCode, envelope, string) {
	t.Helper()
	return runFWAt(t, dir, fw, t.TempDir(), args...)
}

func runFWAt(t *testing.T, dir, fw, root string, args ...string) (ExitCode, envelope, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Run(context.Background(), Env{
		Dir:      dir,
		Stdout:   &out,
		Stderr:   &errb,
		Firmware: fw,
		Environ:  map[string]string{"GROUT_ROOT": root},
	}, args)
	var e envelope
	if err := json.Unmarshal(out.Bytes(), &e); err != nil {
		t.Fatalf("stdout not JSON: %q stderr=%q", out.String(), errb.String())
	}
	return code, e, out.String()
}

func fetchDir(t *testing.T, srv *fakeromm.Server) string {
	t.Helper()
	return fetchDirFW(t, srv, "testfw")
}

func fetchDirFW(t *testing.T, srv *fakeromm.Server, fw string) string {
	t.Helper()
	dir := t.TempDir()
	if err := config.Save(dir, &config.Config{}); err != nil {
		t.Fatal(err)
	}
	code, e, _ := runFW(t, dir, fw, "call", "settings.set_host", fmt.Sprintf(`{"root_uri":%q,"token":"t"}`, srv.URL()))
	if code != ExitOK {
		t.Fatalf("set_host code=%d env=%+v", code, e)
	}
	code, e, _ = runFW(t, dir, fw, "call", "settings.set_directory_mapping", `{"fs_slug":"gba","slug":"gba","enabled":true}`)
	if code != ExitOK {
		t.Fatalf("mapping code=%d env=%+v", code, e)
	}
	code, e, _ = runFW(t, dir, fw, "library", "refresh")
	if code != ExitOK {
		t.Fatalf("refresh code=%d env=%+v", code, e)
	}
	return dir
}

func fetchServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := fakeromm.New(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv.Platforms = []romm.Platform{{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 2, UpdatedAt: base}}
	srv.Roms = fakeromm.Roms(1, 1, 2, base)
	srv.Content = map[int][]byte{}
	srv.Assets = map[string][]byte{}
	for i := range srv.Roms {
		r := &srv.Roms[i]
		body := []byte(fmt.Sprintf("rom data %d", r.ID))
		r.FSName = fmt.Sprintf("game%d.gba", r.ID)
		r.FSNameNoExt = fmt.Sprintf("game%d", r.ID)
		r.FSExtension = "gba"
		r.HasSimpleSingleFile = true
		r.Files = []romm.RomFile{{ID: 100 + r.ID, FileName: r.FSName, FileSizeBytes: int64(len(body))}}
		srv.Content[r.ID] = body
	}
	body := []byte("gba bios bytes")
	sum := md5.Sum(body)
	srv.FirmwareList = []romm.Firmware{{ID: 5, PlatformID: 1, FileName: "gba_bios.bin", SizeBytes: int64(len(body)), MD5: hex.EncodeToString(sum[:]), IsVerified: true}}
	srv.FirmwareContent = map[int][]byte{5: body}
	return srv
}

func intField(t *testing.T, raw json.RawMessage, key string) int {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := json.Unmarshal(m[key], &n); err != nil {
		t.Fatalf("%s in %s: %v", key, raw, err)
	}
	return n
}

func TestFetchGameCommand(t *testing.T) {
	dir := fetchDir(t, fetchServer(t))
	code, e, raw := runCLI(t, dir, "fetch", "game", "1")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	if intField(t, e.Result, "game_id") != 1 || intField(t, e.Result, "file_id") != 101 {
		t.Fatalf("result=%s", e.Result)
	}
}

func TestFetchGameFileFlag(t *testing.T) {
	dir := fetchDir(t, fetchServer(t))
	code, e, raw := runCLI(t, dir, "fetch", "game", "2", "--file", "102")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	if intField(t, e.Result, "game_id") != 2 || intField(t, e.Result, "file_id") != 102 {
		t.Fatalf("result=%s", e.Result)
	}
}

func TestFetchArtCommand(t *testing.T) {
	srv := fetchServer(t)
	const cover = "/assets/romm/resources/roms/1/cover/small.png"
	var img bytes.Buffer
	if err := png.Encode(&img, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	srv.Roms[0].PathCoverSmall = cover
	srv.Assets[cover] = img.Bytes()
	dir := fetchDirFW(t, srv, "knulli")
	root := t.TempDir()
	if code, _, raw := runFWAt(t, dir, "knulli", root, "fetch", "game", "1"); code != ExitOK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	if code, _, raw := runFWAt(t, dir, "knulli", root, "call", "settings.update", `{"download_art":true}`); code != ExitOK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	code, e, raw := runFWAt(t, dir, "knulli", root, "fetch", "art", "1")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	var res struct {
		Written []struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"written"`
		Failed []string `json:"failed"`
	}
	if err := json.Unmarshal(e.Result, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Written) != 1 || res.Written[0].Path == "" || len(res.Failed) != 0 {
		t.Fatalf("result=%s", e.Result)
	}
}

func TestFetchGameArgsJSON(t *testing.T) {
	b, err := json.Marshal(api.FetchGameArgs{GameID: 3})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "file_id") {
		t.Fatalf("args = %s", b)
	}
	b, err = json.Marshal(api.FetchGameArgs{GameID: 3, FileID: 5})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"game_id":3,"file_id":5}` {
		t.Fatalf("args = %s", b)
	}
}

func TestFetchBiosCommand(t *testing.T) {
	dir := fetchDir(t, fetchServer(t))
	code, e, raw := runCLI(t, dir, "fetch", "bios", "1")
	if code != ExitOK || !e.OK {
		t.Fatalf("code=%d env=%s", code, raw)
	}
	var res struct {
		Files []struct {
			FileName string `json:"file_name"`
			Present  bool   `json:"present"`
		} `json:"files"`
	}
	if err := json.Unmarshal(e.Result, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 || res.Files[0].FileName != "gba_bios.bin" || !res.Files[0].Present {
		t.Fatalf("result=%s", e.Result)
	}
}

func runPlain(t *testing.T, dir string, args ...string) ExitCode {
	t.Helper()
	var out, errb bytes.Buffer
	return Run(context.Background(), Env{
		Dir:      dir,
		Stdout:   &out,
		Stderr:   &errb,
		Firmware: "testfw",
		Environ:  map[string]string{"GROUT_ROOT": t.TempDir()},
	}, args)
}

func TestFetchBadIDIsUsage(t *testing.T) {
	for _, args := range [][]string{
		{"fetch", "game", "abc"},
		{"fetch", "game", "0"},
		{"fetch", "game", "-2"},
		{"fetch", "game", "1", "--file", "-1"},
		{"fetch", "art", "abc"},
		{"fetch", "bios", "x"},
		{"fetch", "game"},
	} {
		if code := runPlain(t, t.TempDir(), args...); code != ExitUsage {
			t.Errorf("%v exit = %d, want %d", args, code, ExitUsage)
		}
	}
}

func TestExitForNewCodes(t *testing.T) {
	if exitFor(api.CodeBadArchive) != ExitServer || exitFor(api.CodeMissingOnServer) != ExitServer {
		t.Fatal("new codes are not ExitServer")
	}
}

func TestFetchGameFileFlagHelp(t *testing.T) {
	var game *cobra.Command
	for _, c := range newFetchCmd(Env{}).Commands() {
		if c.Name() == "game" {
			game = c
		}
	}
	if game == nil {
		t.Fatal("no game command")
	}
	want := "pick this version (file id) of a game with several"
	if got := game.Flags().Lookup("file").Usage; got != want {
		t.Fatalf("usage = %q, want %q", got, want)
	}
}

package grout

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

const hostDefinition = `
id = "hostfw"
name = "Host firmware"
maintained = "community"

[paths]
card = { value = "{env.ROOT}" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`

func testOptions(t *testing.T, dataDir string) Options {
	t.Helper()
	return Options{
		DataDir:    dataDir,
		FirmwareID: "testfw",
		Env:        map[string]string{"GROUT_ROOT": t.TempDir()},
	}
}

func open(t *testing.T, o Options) *Grout {
	t.Helper()
	g, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { g.Close() })
	return g
}

func openTest(t *testing.T) *Grout {
	t.Helper()
	return open(t, testOptions(t, t.TempDir()))
}

func wantCode(t *testing.T, err error, want Code) {
	t.Helper()
	if got := CodeOf(err); got != want {
		t.Fatalf("code = %q, want %q (err = %v)", got, want, err)
	}
}

func gbaServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := fakeromm.New(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv.Platforms = []romm.Platform{{ID: 1, Slug: "gba", FSSlug: "gba", Name: "Game Boy Advance", RomCount: 2, UpdatedAt: base}}
	srv.Roms = fakeromm.Roms(1, 1, 2, base)
	return srv
}

func connected(t *testing.T, srv *fakeromm.Server) *Grout {
	t.Helper()
	g := openTest(t)
	if err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := g.SetDirectoryMapping("gba", &config.DirectoryMapping{RomMSlug: "gba"}); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestOpenSecondOpenerIsLocked(t *testing.T) {
	o := testOptions(t, t.TempDir())
	first, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Open(context.Background(), o)
	wantCode(t, err, CodeDataDirLocked)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
}

func TestOpenRequiresExactlyOneFirmware(t *testing.T) {
	neither := testOptions(t, t.TempDir())
	neither.FirmwareID = ""
	_, err := Open(context.Background(), neither)
	wantCode(t, err, CodeBadDefinition)

	both := testOptions(t, t.TempDir())
	both.FirmwareDefinition = []byte(hostDefinition)
	_, err = Open(context.Background(), both)
	wantCode(t, err, CodeBadDefinition)
}

func TestOpenHostDefinitionWithEnvPaths(t *testing.T) {
	dir := t.TempDir()
	g := open(t, Options{
		DataDir:            t.TempDir(),
		FirmwareDefinition: []byte(hostDefinition),
		Env:                map[string]string{"ROOT": dir},
	})
	card := g.Firmware().Card
	if card != dir && !strings.HasPrefix(card, dir+string(filepath.Separator)) {
		t.Fatalf("card = %q, want under %q", card, dir)
	}
}

func TestOpenHostDefinitionMissingEnvIsBadDefinition(t *testing.T) {
	_, err := Open(context.Background(), Options{
		DataDir:            t.TempDir(),
		FirmwareDefinition: []byte(hostDefinition),
		Env:                map[string]string{},
	})
	wantCode(t, err, CodeBadDefinition)
	if !strings.Contains(err.Error(), "ROOT") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenHostDefinitionMissingBiosEnvIsBadDefinition(t *testing.T) {
	def := hostDefinition + `
[platforms]
gba = { roms = ["GBA"], saves = ["GBA"], bios = ["{env.CANNOLI_BIOS}/{rel}"] }
`
	_, err := Open(context.Background(), Options{
		DataDir:            t.TempDir(),
		FirmwareDefinition: []byte(def),
		Env:                map[string]string{"ROOT": t.TempDir()},
	})
	wantCode(t, err, CodeBadDefinition)
	if !strings.Contains(err.Error(), "CANNOLI_BIOS") {
		t.Fatalf("err = %v", err)
	}
}

func TestOpenLogsToDataDir(t *testing.T) {
	dataDir := t.TempDir()
	open(t, testOptions(t, dataDir))
	if _, err := os.Stat(filepath.Join(dataDir, "logs", "grout.log")); err != nil {
		t.Fatal(err)
	}
}

func TestOpenWithoutHostAllowsLocalOperations(t *testing.T) {
	g := openTest(t)
	ctx := context.Background()
	ps, err := g.Platforms(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if ps == nil || len(ps) != 0 {
		t.Fatalf("platforms = %#v", ps)
	}
	if _, err := g.Scan(ctx, nil); err != nil {
		t.Fatal(err)
	}
	_, err = g.Status(ctx)
	wantCode(t, err, CodeNotConfigured)
	_, err = g.Refresh(ctx, RefreshOptions{}, nil)
	wantCode(t, err, CodeNotConfigured)
}

func TestSetHostPersistsAcrossReopen(t *testing.T) {
	srv := fakeromm.New(t)
	o := testOptions(t, t.TempDir())
	g, err := Open(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.SetHost(config.Host{RootURI: srv.URL(), Token: "t"}); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	hosts := open(t, o).Settings().Hosts
	if len(hosts) != 1 || hosts[0].RootURI != srv.URL() || hosts[0].Token != "t" {
		t.Fatalf("hosts = %+v", hosts)
	}
}

func TestSetHostRejectsBadURL(t *testing.T) {
	g := openTest(t)
	for _, uri := range []string{"", "ftp://x"} {
		wantCode(t, g.SetHost(config.Host{RootURI: uri}), CodeBadRequest)
	}
}

func TestSetDirectoryMappingEnablesAndDisables(t *testing.T) {
	srv := gbaServer(t)
	g := connected(t, srv)
	ctx := context.Background()
	if _, err := g.Refresh(ctx, RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	ps, err := g.Platforms(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].FSSlug != "gba" || !ps[0].Enabled {
		t.Fatalf("platforms = %+v", ps)
	}
	if err := g.SetDirectoryMapping("gba", nil); err != nil {
		t.Fatal(err)
	}
	ps, err = g.Platforms(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 0 {
		t.Fatalf("platforms after unmapping = %+v", ps)
	}
	if _, ok := g.Settings().DirectoryMappings["gba"]; ok {
		t.Fatal("mapping still in settings")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	g, err := Open(context.Background(), testOptions(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatalf("second close = %v", err)
	}
	ctx := context.Background()
	_, err = g.Platforms(ctx, false)
	wantCode(t, err, CodeBadRequest)
	_, err = g.Status(ctx)
	wantCode(t, err, CodeBadRequest)
	_, err = g.Refresh(ctx, RefreshOptions{}, nil)
	wantCode(t, err, CodeBadRequest)
	wantCode(t, g.SetHost(config.Host{RootURI: "http://x"}), CodeBadRequest)
}

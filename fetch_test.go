package grout

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/fetch"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/ident"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

const coverAsset = "/assets/romm/resources/roms/1/cover/small.png"

type fetchEnv struct {
	g       *Grout
	srv     *fakeromm.Server
	w       host.Writable
	dataDir string
}

func rootedHost(t *testing.T) host.Writable {
	t.Helper()
	root := t.TempDir()
	return host.OS(func(k string) string {
		if k == "GROUT_ROOT" {
			return root
		}
		return ""
	})
}

func fetchServer(t *testing.T) *fakeromm.Server {
	t.Helper()
	srv := gbaServer(t)
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
	return srv
}

func newFetchEnvWith(t *testing.T, fwID string, srv *fakeromm.Server, h host.Host) *fetchEnv {
	t.Helper()
	dataDir := t.TempDir()
	g := connectedWith(t, srv, Options{DataDir: dataDir, FirmwareID: fwID, Host: h})
	if _, err := g.Refresh(context.Background(), RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	w, _ := h.(host.Writable)
	return &fetchEnv{g: g, srv: srv, w: w, dataDir: dataDir}
}

func newFetchEnv(t *testing.T, fwID string) *fetchEnv {
	t.Helper()
	return newFetchEnvWith(t, fwID, fetchServer(t), rootedHost(t))
}

func (e *fetchEnv) romDir(t *testing.T) string {
	t.Helper()
	dirs := e.g.fw.RomDirs("gba")
	if len(dirs) == 0 {
		t.Fatal("no gba rom dir")
	}
	return dirs[0]
}

func (e *fetchEnv) read(t *testing.T, p string) []byte {
	t.Helper()
	data, err := e.w.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (e *fetchEnv) remove(t *testing.T, p string) {
	t.Helper()
	real, err := e.w.Real(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(real); err != nil {
		t.Fatal(err)
	}
}

func (e *fetchEnv) hold(t *testing.T, gameID int) *gate {
	t.Helper()
	gt := &gate{entered: make(chan struct{}), release: make(chan struct{})}
	e.srv.SetPathHook(romm.RomContentPath(gameID, fmt.Sprintf("game%d.gba", gameID)), func() {
		gt.enter.Do(func() { close(gt.entered) })
		<-gt.release
	})
	t.Cleanup(gt.unblock)
	return gt
}

func pngData(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

type fetchOutcome struct {
	res FetchGameResult
	err error
}

func fetchAsync(g *Grout, gameID int) <-chan fetchOutcome {
	out := make(chan fetchOutcome, 1)
	go func() {
		res, err := g.FetchGame(context.Background(), gameID, 0, nil)
		out <- fetchOutcome{res, err}
	}()
	return out
}

func wantMessage(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil || err.Error() != want {
		t.Fatalf("err = %v, want %q", err, want)
	}
}

func artKinds(r fetch.ArtResult) []firmware.ArtKind {
	out := []firmware.ArtKind{}
	for _, f := range r.Written {
		out = append(out, f.Kind)
	}
	return out
}

func TestFetchGameRecordsAndArt(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "knulli")
	e.srv.Mutate(func() {
		e.srv.Roms[0].PathCoverSmall = coverAsset
		e.srv.Assets[coverAsset] = pngData(t)
	})
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	stages := map[library.Stage]bool{}
	res, err := e.g.FetchGame(ctx, 1, 0, func(p library.Progress) {
		mu.Lock()
		stages[p.Stage] = true
		mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := e.romDir(t)
	launch := path.Join(dir, "game1.gba")
	if res.GameID != 1 || res.FileID != 101 || res.RomDir != dir || res.LaunchPath != launch || !reflect.DeepEqual(res.Placed, []string{launch}) {
		t.Fatalf("result = %+v", res.GameResult)
	}
	if got := e.read(t, launch); string(got) != "rom data 1" {
		t.Fatalf("content = %q", got)
	}
	if !stages[library.StageDownload] || !stages[library.StageArt] {
		t.Fatalf("stages = %v", stages)
	}
	if len(res.Art.Failed) != 0 || !reflect.DeepEqual(artKinds(res.Art), []firmware.ArtKind{firmware.ArtCover}) {
		t.Fatalf("art = %+v", res.Art)
	}
	cover := res.Art.Written[0].Path
	if want := path.Join(dir, "images", e.g.fw.ArtFileName(firmware.ArtCover, "game1")); cover != want {
		t.Fatalf("cover = %q, want %q", cover, want)
	}
	e.read(t, cover)
	state, err := e.g.GameState(ctx, 1)
	if err != nil || state != library.Downloaded {
		t.Fatalf("state = %v, err = %v", state, err)
	}

	art, err := e.g.FetchArt(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Written) != 0 || len(art.Failed) != 0 {
		t.Fatalf("art with cover present = %+v", art)
	}
	e.remove(t, cover)
	art, err = e.g.FetchArt(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(artKinds(art.ArtResult), []firmware.ArtKind{firmware.ArtCover}) || art.Written[0].Path != cover {
		t.Fatalf("art after removal = %+v", art)
	}
	e.read(t, cover)

	e.srv.FailPath("/api/roms/2")
	res, err = e.g.FetchGame(ctx, 2, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Art.Written) != 0 || !reflect.DeepEqual(res.Art.Failed, []firmware.ArtKind{firmware.ArtCover}) {
		t.Fatalf("art on detail failure = %+v", res.Art)
	}
	if state, _ := e.g.GameState(ctx, 2); state != library.Downloaded {
		t.Fatalf("state = %v", state)
	}
}

func TestFetchGameWithoutArt(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Art.Written == nil || res.Art.Failed == nil || len(res.Art.Written)+len(res.Art.Failed) != 0 {
		t.Fatalf("art = %#v", res.Art)
	}
	if len(e.srv.CallsTo("/api/roms/1")) != 0 {
		t.Fatal("rom detail fetched with art off")
	}
}

func TestFetchGameUnknownGame(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	_, err := e.g.FetchGame(ctx, 99, 0, nil)
	wantCode(t, err, CodeBadRequest)
	_, err = e.g.FetchGame(ctx, 1, 555, nil)
	wantCode(t, err, CodeBadRequest)
	_, err = e.g.FetchArt(ctx, 99, nil)
	wantCode(t, err, CodeBadRequest)
	_, err = e.g.GameFiles(ctx, 99)
	wantCode(t, err, CodeBadRequest)
	files, err := e.g.GameFiles(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []library.GameFile{{ID: 101, FileName: "game1.gba", Size: 10}}; !reflect.DeepEqual(files, want) {
		t.Fatalf("files = %+v", files)
	}
}

func TestFetchGameNeedsToken(t *testing.T) {
	ctx := context.Background()
	g := openTest(t)
	_, err := g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeNotConfigured)
	e := newFetchEnv(t, "testfw")
	if err := e.g.Logout(); err != nil {
		t.Fatal(err)
	}
	_, err = e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeNotConfigured)
	_, err = e.g.FetchArt(ctx, 1, nil)
	wantCode(t, err, CodeNotConfigured)
	_, err = e.g.FetchBios(ctx, 1, nil)
	wantCode(t, err, CodeNotConfigured)
	_, err = e.g.LibraryBios(ctx, 1)
	wantCode(t, err, CodeNotConfigured)
}

func TestFetchGameWhileRefreshIsBusy(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	gt := block(t, e.srv)
	done := refreshAsync(e.g, ctx)
	<-gt.entered

	_, err := e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeBusy)
	wantMessage(t, err, "a refresh, scan or download is running")
	_, err = e.g.FetchBios(ctx, 1, nil)
	wantCode(t, err, CodeBusy)
	_, err = e.g.FetchArt(ctx, 1, nil)
	wantCode(t, err, CodeBusy)
	if _, err := e.g.LibraryBios(ctx, 1); err != nil {
		t.Fatal(err)
	}

	gt.unblock()
	if out := <-done; out.err != nil {
		t.Fatal(out.err)
	}
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshWhileFetchIsBusy(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	gt := e.hold(t, 1)
	done := fetchAsync(e.g, 1)
	<-gt.entered

	_, err := e.g.Refresh(ctx, RefreshOptions{}, nil)
	wantCode(t, err, CodeBusy)
	wantMessage(t, err, "a refresh, scan or download is running")
	_, err = e.g.Scan(ctx, nil)
	wantCode(t, err, CodeBusy)
	if _, err := e.g.Games(ctx, library.Query{}); err != nil {
		t.Fatal(err)
	}

	gt.unblock()
	if out := <-done; out.err != nil {
		t.Fatal(out.err)
	}
	if _, err := e.g.Refresh(ctx, RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestTwoGamesFetchTogether(t *testing.T) {
	e := newFetchEnv(t, "testfw")
	e.srv.SetContentDelay(50 * time.Millisecond)
	var arrived sync.WaitGroup
	arrived.Add(2)
	both := make(chan struct{})
	go func() {
		arrived.Wait()
		close(both)
	}()
	for id := 1; id <= 2; id++ {
		e.srv.SetPathHook(romm.RomContentPath(id, fmt.Sprintf("game%d.gba", id)), func() {
			arrived.Done()
			select {
			case <-both:
			case <-time.After(5 * time.Second):
			}
		})
	}
	first, second := fetchAsync(e.g, 1), fetchAsync(e.g, 2)
	for _, ch := range []<-chan fetchOutcome{first, second} {
		if out := within(t, ch, "FetchGame"); out.err != nil {
			t.Fatal(out.err)
		}
	}
	select {
	case <-both:
	default:
		t.Fatal("downloads did not overlap")
	}
	dir := e.romDir(t)
	if string(e.read(t, path.Join(dir, "game1.gba"))) != "rom data 1" || string(e.read(t, path.Join(dir, "game2.gba"))) != "rom data 2" {
		t.Fatal("content mismatch")
	}
}

func TestSameGameTwiceIsBusy(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	gt := e.hold(t, 1)
	done := fetchAsync(e.g, 1)
	<-gt.entered

	_, err := e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeBusy)
	wantMessage(t, err, "this item is already downloading")
	_, err = e.g.FetchArt(ctx, 1, nil)
	wantCode(t, err, CodeBusy)
	wantMessage(t, err, "this item is already downloading")

	gt.unblock()
	if out := <-done; out.err != nil {
		t.Fatal(out.err)
	}
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSameBiosTwiceIsBusy(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	body := []byte("bios")
	e.srv.Mutate(func() {
		e.srv.FirmwareList = []romm.Firmware{{ID: 5, PlatformID: 1, FileName: "gba_bios.bin", SizeBytes: int64(len(body))}}
		e.srv.FirmwareContent = map[int][]byte{5: body}
	})
	gt := &gate{entered: make(chan struct{}), release: make(chan struct{})}
	e.srv.SetPathHook(romm.FirmwareContentPath(5, "gba_bios.bin"), func() {
		gt.enter.Do(func() { close(gt.entered) })
		<-gt.release
	})
	t.Cleanup(gt.unblock)
	done := make(chan error, 1)
	go func() {
		_, err := e.g.FetchBios(ctx, 1, nil)
		done <- err
	}()
	<-gt.entered
	_, err := e.g.FetchBios(ctx, 1, nil)
	wantCode(t, err, CodeBusy)
	wantMessage(t, err, "this item is already downloading")
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
	gt.unblock()
	if err := within(t, done, "FetchBios"); err != nil {
		t.Fatal(err)
	}
}

func TestFetchArtNotDownloaded(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	_, err := e.g.FetchArt(context.Background(), 1, nil)
	wantCode(t, err, CodeBadRequest)
}

func TestFetchArtStem(t *testing.T) {
	dirs := []string{"/roms/gba", "/roms/psx/"}
	for launch, want := range map[string]struct {
		folder    bool
		dir, stem string
	}{
		"/roms/gba/game1.gba":        {false, "/roms/gba", "game1"},
		"/roms/gba/Game.Name.v1.zip": {false, "/roms/gba", "Game.Name.v1"},
		"/roms/psx/Game.m3u":         {false, "/roms/psx", "Game"},
		"/roms/psx/Game/Game.m3u":    {false, "/roms/psx", "Game"},
		"/roms/psx/_Game Two":        {true, "/roms/psx", "_Game Two"},
		"/other/snes/Game.sfc":       {false, "/other/snes", "Game"},
		"/roms/psx/.hidden/Game.m3u": {false, "/roms/psx/.hidden", "Game"},
	} {
		dir, stem := artTarget(dirs, launch, want.folder)
		if dir != want.dir || stem != want.stem {
			t.Errorf("%s: dir %q stem %q, want %q %q", launch, dir, stem, want.dir, want.stem)
		}
	}
}

func TestFetchBiosAndLibraryBios(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	body := []byte("gba bios bytes")
	sum := md5.Sum(body)
	e.srv.Mutate(func() {
		e.srv.FirmwareList = []romm.Firmware{{ID: 5, PlatformID: 1, FileName: "gba_bios.bin", SizeBytes: int64(len(body)), MD5: hex.EncodeToString(sum[:]), IsVerified: true}}
		e.srv.FirmwareContent = map[int][]byte{5: body}
	})
	before, err := e.g.LibraryBios(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || before[0].Present || before[0].FileName != "gba_bios.bin" || len(before[0].Paths) == 0 {
		t.Fatalf("before = %+v", before)
	}
	got, err := e.g.FetchBios(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Present || got[0].Check != fetch.BiosVerified || !got[0].IsVerified {
		t.Fatalf("fetched = %+v", got)
	}
	for _, p := range got[0].Paths {
		if string(e.read(t, p)) != string(body) {
			t.Fatalf("%s mismatch", p)
		}
	}
	after, err := e.g.LibraryBios(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || !after[0].Present || !reflect.DeepEqual(after[0].Paths, got[0].Paths) {
		t.Fatalf("after = %+v", after)
	}
	_, err = e.g.FetchBios(ctx, 42, nil)
	wantCode(t, err, CodeBadRequest)
	_, err = e.g.LibraryBios(ctx, 42)
	wantCode(t, err, CodeBadRequest)
}

type readOnlyHost struct {
	host.Host
}

func TestFetchReadOnlyHost(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnvWith(t, "testfw", fetchServer(t), readOnlyHost{rootedHost(t)})
	_, err := e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeInternal)
	wantMessage(t, err, "host is read-only")
	_, err = e.g.FetchArt(ctx, 1, nil)
	wantCode(t, err, CodeInternal)
	_, err = e.g.FetchBios(ctx, 1, nil)
	wantCode(t, err, CodeInternal)
	if _, err := e.g.LibraryBios(ctx, 1); err != nil {
		t.Fatal(err)
	}
}

func TestFetchMissingOnServer(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	e.srv.Mutate(func() {
		delete(e.srv.Content, 1)
		e.srv.FirmwareList = []romm.Firmware{{ID: 5, PlatformID: 1, FileName: "gba_bios.bin"}}
	})
	_, err := e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeMissingOnServer)
	files, err := e.g.FetchBios(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Check != fetch.BiosUnchecked || files[0].Present {
		t.Fatalf("bios = %+v", files)
	}
	if state, _ := e.g.GameState(ctx, 1); state != library.NotDownloaded {
		t.Fatalf("state = %v", state)
	}
}

func TestFetchBadArchive(t *testing.T) {
	ctx := context.Background()
	srv := fetchServer(t)
	body := []byte("PK\x03\x04not really a zip")
	srv.Roms[0].FSName = "game1.zip"
	srv.Roms[0].Files = []romm.RomFile{{ID: 101, FileName: "game1.zip", FileSizeBytes: int64(len(body))}}
	srv.Content[1] = body
	e := newFetchEnvWith(t, "testfw", srv, rootedHost(t))
	if _, err := e.g.UpdateSettings(SettingsPatch{UnzipDownloads: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	_, err := e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeBadArchive)
	if !errors.Is(err, fetch.ErrUnsafeArchive) {
		t.Fatalf("err = %v", err)
	}
}

func TestFetchErrorCodes(t *testing.T) {
	ctx := context.Background()
	for name, tc := range map[string]struct {
		err  error
		want Code
	}{
		"no rom dir":   {fmt.Errorf("gba: %w", fetch.ErrNoRomDir), CodeBadRequest},
		"bad file":     {fetch.ErrBadFile, CodeBadRequest},
		"multi file":   {fetch.ErrFileForMulti, CodeBadRequest},
		"no files":     {fetch.ErrNoFiles, CodeBadRequest},
		"unsafe":       {fmt.Errorf("%w: x", fetch.ErrUnsafeArchive), CodeBadArchive},
		"not found":    {&romm.StatusError{Code: 404}, CodeMissingOnServer},
		"server error": {&romm.StatusError{Code: 500}, CodeServerUnreachable},
		"truncated":    {fmt.Errorf("reading: %w", io.ErrUnexpectedEOF), CodeServerUnreachable},
		"no space":     {fmt.Errorf("writing: %w", syscall.ENOSPC), CodeStorage},
		"path":         {&os.PathError{Op: "open", Path: "/x", Err: syscall.EACCES}, CodeStorage},
		"link":         {&os.LinkError{Op: "rename", Old: "/a", New: "/b", Err: syscall.EXDEV}, CodeStorage},
		"other":        {errors.New("boom"), CodeInternal},
	} {
		t.Run(name, func(t *testing.T) {
			wantCode(t, fetchError(ctx, tc.err), tc.want)
		})
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	wantCode(t, fetchError(cancelled, &romm.StatusError{Code: 404}), CodeCancelled)
}

func zipOf(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		f, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("data " + n)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func noStagingUnder(t *testing.T, w host.Writable) {
	t.Helper()
	root, err := w.Real("/")
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(d.Name(), fetch.StagePrefix) {
			t.Errorf("staging left behind: %s", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFetchGameTruncatedTransfer(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "testfw")
	e.srv.SetContentLength(1, 4096)
	_, err := e.g.FetchGame(ctx, 1, 0, nil)
	wantCode(t, err, CodeServerUnreachable)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("err = %v", err)
	}
	noStagingUnder(t, e.w)
	if _, err := e.w.Stat(path.Join(e.romDir(t), "game1.gba")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("final file present: %v", err)
	}
	if state, _ := e.g.GameState(ctx, 1); state != library.NotDownloaded {
		t.Fatalf("state = %v", state)
	}
}

type statFailHost struct {
	host.Writable
	fail string
}

func (h statFailHost) Stat(p string) (fs.FileInfo, error) {
	if p == h.fail {
		return nil, errors.New("stat unavailable")
	}
	return h.Writable.Stat(p)
}

func TestFetchGameDottedFolderStem(t *testing.T) {
	srv := fetchServer(t)
	r := &srv.Roms[0]
	r.FSName, r.FSNameNoExt, r.FSExtension = "Game v1.2", "Game v1.2", ""
	r.HasSimpleSingleFile, r.HasMultipleFiles = false, true
	r.PathCoverSmall = coverAsset
	r.Files = []romm.RomFile{{ID: 101, FileName: "disc1.bin", FileSizeBytes: 15}, {ID: 102, FileName: "disc2.bin", FileSizeBytes: 15}}
	srv.Content[1] = zipOf(t, "disc1.bin", "disc2.bin")
	srv.Assets[coverAsset] = pngData(t)
	w := rootedHost(t)
	h := &statFailHost{Writable: w}
	e := newFetchEnvWith(t, "knulli", srv, h)
	folder := path.Join(e.romDir(t), "Game v1.2")
	h.fail = folder
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.LaunchPath != folder {
		t.Fatalf("launch = %q", res.LaunchPath)
	}
	want := path.Join(e.romDir(t), "images", e.g.fw.ArtFileName(firmware.ArtCover, "Game v1.2"))
	if len(res.Art.Failed) != 0 || len(res.Art.Written) != 1 || res.Art.Written[0].Path != want {
		t.Fatalf("art = %+v, want cover at %q", res.Art, want)
	}
}

type cancelOnCommitHost struct {
	host.Writable
	mu     sync.Mutex
	target string
	cancel context.CancelFunc
}

func (h *cancelOnCommitHost) Real(p string) (string, error) {
	h.mu.Lock()
	if p == h.target && h.cancel != nil {
		h.cancel()
		h.cancel = nil
	}
	h.mu.Unlock()
	return h.Writable.Real(p)
}

func TestFetchGameCancelAfterPlacement(t *testing.T) {
	srv := fetchServer(t)
	srv.Roms[0].PathCoverSmall = coverAsset
	srv.Assets[coverAsset] = pngData(t)
	h := &cancelOnCommitHost{Writable: rootedHost(t)}
	e := newFetchEnvWith(t, "knulli", srv, h)
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true), LogLevel: ptr(config.LogLevelDebug)}); err != nil {
		t.Fatal(err)
	}
	launch := path.Join(e.romDir(t), "game1.gba")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.mu.Lock()
	h.target, h.cancel = launch, cancel
	h.mu.Unlock()
	res, err := e.g.FetchGame(ctx, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("context was not cancelled during placement")
	}
	if res.LaunchPath != launch || string(e.read(t, launch)) != "rom data 1" {
		t.Fatalf("result = %+v", res.GameResult)
	}
	if len(res.Art.Written) != 0 || !reflect.DeepEqual(res.Art.Failed, []firmware.ArtKind{firmware.ArtCover}) {
		t.Fatalf("art = %+v", res.Art)
	}
	if len(e.srv.CallsTo("/api/roms/1")) != 0 {
		t.Fatal("rom detail fetched after cancel")
	}
	if state, err := e.g.GameState(context.Background(), 1); err != nil || state != library.Downloaded {
		t.Fatalf("state = %v, err = %v", state, err)
	}
	logged, err := os.ReadFile(filepath.Join(e.dataDir, "logs", "grout.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logged), "art details unavailable") {
		t.Fatalf("warning logged after cancel:\n%s", logged)
	}
}

func TestFetchArtMultiInGameFolder(t *testing.T) {
	ctx := context.Background()
	srv := fetchServer(t)
	r := &srv.Roms[0]
	r.FSName, r.FSNameNoExt, r.FSExtension = "Game", "Game", ""
	r.HasSimpleSingleFile, r.HasMultipleFiles = false, true
	r.PathCoverSmall = coverAsset
	r.Files = []romm.RomFile{{ID: 101, FileName: "Game (Disc 1).cue", FileSizeBytes: 15}, {ID: 102, FileName: "Game (Disc 2).cue", FileSizeBytes: 15}}
	srv.Content[1] = zipOf(t, "Game.m3u", "Game (Disc 1).cue", "Game (Disc 2).cue")
	srv.Assets[coverAsset] = pngData(t)
	e := newFetchEnvWith(t, "knulli", srv, rootedHost(t))
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	res, err := e.g.FetchGame(ctx, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := path.Join(e.romDir(t), "Game", "Game.m3u"); res.LaunchPath != want {
		t.Fatalf("launch = %q, want %q", res.LaunchPath, want)
	}
	cover := path.Join(e.romDir(t), "images", e.g.fw.ArtFileName(firmware.ArtCover, "Game"))
	if len(res.Art.Written) != 1 || res.Art.Written[0].Path != cover {
		t.Fatalf("art = %+v, want cover at %q", res.Art, cover)
	}
	art, err := e.g.FetchArt(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Written) != 0 || len(art.Failed) != 0 {
		t.Fatalf("art with cover present = %+v", art)
	}
	e.remove(t, cover)
	art, err = e.g.FetchArt(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(art.Written) != 1 || art.Written[0].Path != cover {
		t.Fatalf("art after removal = %+v, want cover at %q", art, cover)
	}
	e.read(t, cover)
}

func TestFetchArtGameGoneFromCard(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "knulli")
	e.srv.Mutate(func() {
		e.srv.Roms[0].PathCoverSmall = coverAsset
		e.srv.Assets[coverAsset] = pngData(t)
	})
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	e.remove(t, path.Join(e.romDir(t), "game1.gba"))
	art, err := e.g.FetchArt(ctx, 1, nil)
	wantCode(t, err, CodeBadRequest)
	wantMessage(t, err, "game 1 is not on the card")
	if len(art.Written) != 0 {
		t.Fatalf("art = %+v", art)
	}
}

func TestFetchGameSkippedArtOnlyKindsWithDir(t *testing.T) {
	ctx := context.Background()
	e := newFetchEnv(t, "nextui")
	if _, err := e.g.UpdateSettings(SettingsPatch{DownloadArt: ptr(true), AdditionalDownloads: &AdditionalDownloadsPatch{Video: ptr(true)}}); err != nil {
		t.Fatal(err)
	}
	e.srv.FailPath("/api/roms/1")
	res, err := e.g.FetchGame(ctx, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Art.Written) != 0 || !reflect.DeepEqual(res.Art.Failed, []firmware.ArtKind{firmware.ArtCover}) {
		t.Fatalf("art = %+v", res.Art)
	}
}

func pspServer(t *testing.T, name string, body []byte) *fakeromm.Server {
	t.Helper()
	srv := fetchServer(t)
	srv.Platforms = []romm.Platform{{ID: 1, Slug: "psp", FSSlug: "psp", Name: "PlayStation Portable", RomCount: 2, UpdatedAt: srv.Platforms[0].UpdatedAt}}
	for i := range srv.Roms {
		r := &srv.Roms[i]
		r.FSName = name
		r.FSNameNoExt = strings.TrimSuffix(name, ".iso")
		r.FSExtension = "iso"
		r.Files = []romm.RomFile{{ID: 100 + r.ID, FileName: name, FileSizeBytes: int64(len(body))}}
		srv.Content[r.ID] = body
	}
	return srv
}

type identityRow struct {
	status  string
	titleID string
}

func identityRows(t *testing.T, e *fetchEnv) map[string]identityRow {
	t.Helper()
	rows, err := e.g.store.DB.Query(`SELECT path, status, title_id FROM identities`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]identityRow{}
	for rows.Next() {
		var p string
		var r identityRow
		if err := rows.Scan(&p, &r.status, &r.titleID); err != nil {
			t.Fatal(err)
		}
		out[p] = r
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func newPspEnv(t *testing.T, name string) *fetchEnv {
	t.Helper()
	srv := pspServer(t, name, make([]byte, 64*1024))
	dataDir := t.TempDir()
	h := rootedHost(t)
	g := connectedWith(t, srv, Options{DataDir: dataDir, FirmwareID: "knulli", Host: h})
	if err := g.SetDirectoryMapping("psp", &config.DirectoryMapping{RomMSlug: "psp"}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Refresh(context.Background(), RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	w, _ := h.(host.Writable)
	return &fetchEnv{g: g, srv: srv, w: w, dataDir: dataDir}
}

func TestFetchGameIdentifiesTheDownload(t *testing.T) {
	e := newPspEnv(t, "Game [ULUS10565].iso")
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	row, ok := identityRows(t, e)[res.LaunchPath]
	if !ok || row.titleID != "ULUS10565" {
		t.Fatalf("identities = %+v, launch = %q", identityRows(t, e), res.LaunchPath)
	}
}

func TestFetchGameIgnoresIdentificationFailure(t *testing.T) {
	e := newPspEnv(t, "Game.iso")
	res, err := e.g.FetchGame(context.Background(), 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	launch := path.Join(e.g.fw.RomDirs("psp")[0], "Game.iso")
	if res.LaunchPath != launch || res.GameID != 1 {
		t.Fatalf("result = %+v", res.GameResult)
	}
	if len(res.Art.Failed) != 0 || len(res.Gamelist.Failed) != 0 {
		t.Fatalf("art = %+v gamelist = %+v", res.Art, res.Gamelist)
	}
	row, ok := identityRows(t, e)[launch]
	if !ok || row.status == "ok" {
		t.Fatalf("identities = %+v", identityRows(t, e))
	}
}

func TestFetchGameOnUnmappedPlatformStoresNoIdentity(t *testing.T) {
	e := newFetchEnv(t, "knulli")
	if _, err := e.g.FetchGame(context.Background(), 1, 0, nil); err != nil {
		t.Fatal(err)
	}
	if rows := identityRows(t, e); len(rows) != 0 {
		t.Fatalf("identities = %+v", rows)
	}
}

type cancelOnExtract struct {
	ident.Extractor
	cancel context.CancelFunc
}

func (c cancelOnExtract) Extract(p, platform string) ident.Result {
	c.cancel()
	return c.Extractor.Extract(p, platform)
}

func TestFetchGameCancelledDuringIdentifyMatchesCancelledFetch(t *testing.T) {
	e := newPspEnv(t, "Game [ULUS10565].iso")
	if _, err := e.g.UpdateSettings(SettingsPatch{LogLevel: ptr(config.LogLevelDebug)}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e.g.ident = cancelOnExtract{Extractor: e.g.ident, cancel: cancel}
	res, err := e.g.FetchGame(ctx, 1, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Err() == nil {
		t.Fatal("context was not cancelled during identification")
	}
	if len(res.Art.Written) != 0 {
		t.Fatalf("art = %+v", res.Art)
	}
	if !reflect.DeepEqual(res.Gamelist, emptyGamelist()) {
		t.Fatalf("gamelist = %+v", res.Gamelist)
	}
	logged, err := os.ReadFile(filepath.Join(e.dataDir, "logs", "grout.log"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(logged), "identify failed") || strings.Contains(string(logged), "game fetched") {
		t.Fatalf("warning logged after cancel:\n%s", logged)
	}
}

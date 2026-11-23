package fetch

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/library"
	"github.com/rommapp/grout/core/romm"
)

func TestWantedOff(t *testing.T) {
	c := config.Config{ArtKind: config.ArtKindDefault, DownloadArtScreenshotPreview: true, AdditionalDownloads: config.AdditionalDownloads{Video: true}}
	if got := Wanted(c); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestWantedTable(t *testing.T) {
	spec := func(s romm.ArtSource, k config.ArtKind) romm.ArtSourceSpec {
		return romm.ArtSourceSpec{Source: s, Kind: k}
	}
	cases := []struct {
		name string
		in   config.Config
		want map[firmware.ArtKind]romm.ArtSourceSpec
	}{
		{"nothing else", config.Config{DownloadArt: true}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover: spec(romm.SourceCover, config.ArtKindNone),
		}},
		{"cover kind", config.Config{DownloadArt: true, ArtKind: config.ArtKindBox3D}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover: spec(romm.SourceCover, config.ArtKindBox3D),
		}},
		{"preview", config.Config{DownloadArt: true, ArtKind: config.ArtKindDefault, DownloadArtScreenshotPreview: true}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover:   spec(romm.SourceCover, config.ArtKindDefault),
			firmware.ArtPreview: spec(romm.SourceScreenshot, config.ArtKindNone),
		}},
		{"splash only", config.Config{DownloadArt: true, ArtKind: config.ArtKindDefault, DownloadSplashArt: config.ArtKindTitle}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover:     spec(romm.SourceCover, config.ArtKindDefault),
			firmware.ArtSplash:    spec(romm.SourceSplash, config.ArtKindTitle),
			firmware.ArtThumbnail: spec(romm.SourceSplash, config.ArtKindTitle),
		}},
		{"thumbnail overrides splash", config.Config{DownloadArt: true, ArtKind: config.ArtKindDefault, DownloadSplashArt: config.ArtKindTitle,
			AdditionalDownloads: config.AdditionalDownloads{Thumbnail: config.ArtKindMarquee}}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover:     spec(romm.SourceCover, config.ArtKindDefault),
			firmware.ArtSplash:    spec(romm.SourceSplash, config.ArtKindMarquee),
			firmware.ArtThumbnail: spec(romm.SourceSplash, config.ArtKindMarquee),
		}},
		{"thumbnail alone", config.Config{DownloadArt: true, ArtKind: config.ArtKindDefault,
			AdditionalDownloads: config.AdditionalDownloads{Thumbnail: config.ArtKindTitle}}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover:     spec(romm.SourceCover, config.ArtKindDefault),
			firmware.ArtSplash:    spec(romm.SourceSplash, config.ArtKindTitle),
			firmware.ArtThumbnail: spec(romm.SourceSplash, config.ArtKindTitle),
		}},
		{"everything", config.Config{DownloadArt: true, ArtKind: config.ArtKindDefault,
			AdditionalDownloads: config.AdditionalDownloads{Marquee: config.ArtKindLogo, Video: true, Bezel: true, Manual: true, BoxBack: true, Fanart: true}}, map[firmware.ArtKind]romm.ArtSourceSpec{
			firmware.ArtCover:   spec(romm.SourceCover, config.ArtKindDefault),
			firmware.ArtMarquee: spec(romm.SourceMarquee, config.ArtKindLogo),
			firmware.ArtVideo:   spec(romm.SourceVideo, config.ArtKindNone),
			firmware.ArtBezel:   spec(romm.SourceBezel, config.ArtKindNone),
			firmware.ArtManual:  spec(romm.SourceManual, config.ArtKindNone),
			firmware.ArtBoxback: spec(romm.SourceBoxback, config.ArtKindNone),
			firmware.ArtFanart:  spec(romm.SourceFanart, config.ArtKindNone),
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Wanted(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got  %v\nwant %v", got, tc.want)
			}
		})
	}
}

type artEnv struct {
	*gameEnv
	external *httptest.Server
	extMu    sync.Mutex
	extAuth  []string
	extPaths []string
	extData  map[string][]byte
}

func newArtEnv(t *testing.T, fwID string) *artEnv {
	t.Helper()
	e := &artEnv{gameEnv: newGameEnv(t, fwID, "gba"), extData: map[string][]byte{}}
	e.external = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.extMu.Lock()
		e.extAuth = append(e.extAuth, r.Header.Get("Authorization"))
		e.extPaths = append(e.extPaths, r.URL.Path)
		data, ok := e.extData[r.URL.Path]
		e.extMu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(data)
	}))
	t.Cleanup(e.external.Close)
	e.srv.Assets = map[string][]byte{}
	return e
}

const rommAsset = "/assets/romm/resources/roms/1/"

func (e *artEnv) detail(t *testing.T, d romm.RomDetail) romm.RomDetail {
	t.Helper()
	d.ID = 1
	e.srv.Details = map[int]romm.RomDetail{1: d}
	out, err := e.srv.Client.RomDetail(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func (e *artEnv) input(d romm.RomDetail, c config.Config) ArtInput {
	return ArtInput{
		Detail:   d,
		Platform: library.Platform{ID: 7, Slug: "gba", Name: "GBA", RomDirs: []string{e.romDir}},
		RomDir:   e.romDir,
		Stem:     "Game",
		Settings: c,
		Client:   e.srv.Client,
		Firmware: e.fw,
		Host:     e.w,
	}
}

func (e *artEnv) read(t *testing.T, card string) []byte {
	t.Helper()
	real, err := e.w.Real(card)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func (e *artEnv) exists(t *testing.T, card string) bool {
	t.Helper()
	real, err := e.w.Real(card)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(real)
	return err == nil
}

func baseCfg() config.Config {
	return config.Config{DownloadArt: true, ArtKind: config.ArtKindDefault}
}

func writtenKinds(r ArtResult) []firmware.ArtKind {
	var out []firmware.ArtKind
	for _, f := range r.Written {
		out = append(out, f.Kind)
	}
	return out
}

func TestArtWritesNamedFiles(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 40, 20)
	e.srv.Assets[rommAsset+"marquee.png"] = pngBytes(t, 30, 10)
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("VIDEODATA")
	e.srv.Assets[rommAsset+"manual.pdf"] = []byte("%PDF-manual")
	e.extData["/title.png"] = pngBytes(t, 16, 16)
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png", HasManual: true},
		PathManual: rommAsset + "manual.pdf",
		SSMetadata: &romm.SSMetadata{MarqueePath: rommAsset + "marquee.png", VideoPath: rommAsset + "video.mp4", TitleScreenURL: e.external.URL + "/title.png"},
	})
	c := baseCfg()
	c.AdditionalDownloads = config.AdditionalDownloads{Marquee: config.ArtKindMarquee, Video: true, Manual: true, Thumbnail: config.ArtKindTitle}
	in := e.input(d, c)
	in.MaxSize = Size{Width: 20, Height: 20}
	var rec recorder
	res := Art(context.Background(), in, rec.add)
	if len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	want := []firmware.ArtKind{firmware.ArtCover, firmware.ArtVideo, firmware.ArtManual, firmware.ArtMarquee, firmware.ArtThumbnail}
	if got := writtenKinds(res); !reflect.DeepEqual(got, want) {
		t.Fatalf("kinds %v want %v", got, want)
	}
	dir := path.Join(e.romDir, "images")
	paths := map[firmware.ArtKind]string{
		firmware.ArtCover:     path.Join(dir, "Game.png"),
		firmware.ArtMarquee:   path.Join(dir, "Game-marquee.png"),
		firmware.ArtThumbnail: path.Join(dir, "Game-thumb.png"),
		firmware.ArtVideo:     path.Join(e.romDir, "videos", "Game.mp4"),
		firmware.ArtManual:    path.Join(e.romDir, "manuals", "Game.pdf"),
	}
	for _, f := range res.Written {
		if f.Path != paths[f.Kind] {
			t.Fatalf("%s path %q want %q", f.Kind, f.Path, paths[f.Kind])
		}
		if !e.exists(t, f.Path) {
			t.Fatalf("%s missing", f.Path)
		}
	}
	if w, h := decodedSize(t, e.read(t, paths[firmware.ArtCover])); w != 20 || h != 10 {
		t.Fatalf("cover %dx%d", w, h)
	}
	if string(e.read(t, paths[firmware.ArtVideo])) != "VIDEODATA" || string(e.read(t, paths[firmware.ArtManual])) != "%PDF-manual" {
		t.Fatal("video and manual must be written as received")
	}
	last := rec.events[len(rec.events)-1]
	if last.Stage != library.StageArt || last.Done != 5 || last.Total != 5 {
		t.Fatalf("last progress %+v", last)
	}
	for _, p := range rec.events {
		if p.Stage != library.StageArt || p.Total != 5 {
			t.Fatalf("progress %+v", p)
		}
	}
	realRoot, _ := e.w.Real(e.romDir)
	filepath.WalkDir(realRoot, func(p string, d os.DirEntry, err error) error {
		if err == nil && strings.HasPrefix(d.Name(), StagePrefix) {
			t.Errorf("staging file left behind: %s", p)
		}
		return nil
	})
}

func TestArtSkipsKindsWithoutDir(t *testing.T) {
	e := newArtEnv(t, "nextui")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("V")
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png"},
		SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"},
	})
	c := baseCfg()
	c.AdditionalDownloads.Video = true
	res := Art(context.Background(), e.input(d, c), nil)
	if got := writtenKinds(res); !reflect.DeepEqual(got, []firmware.ArtKind{firmware.ArtCover}) || len(res.Failed) != 0 {
		t.Fatalf("written %v failed %v", got, res.Failed)
	}
	if res.Written[0].Path != path.Join(e.romDir, ".media", "Game.png") {
		t.Fatalf("path %q", res.Written[0].Path)
	}
}

func TestArtFailureDoesNotStopOthers(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("V")
	e.srv.Assets[rommAsset+"manual.pdf"] = []byte("M")
	e.srv.Assets[rommAsset+"marquee.png"] = []byte("this is not an image")
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png", HasManual: true},
		PathManual: rommAsset + "manual.pdf",
		SSMetadata: &romm.SSMetadata{MarqueePath: rommAsset + "marquee.png", VideoPath: rommAsset + "video.mp4"},
	})
	c := baseCfg()
	c.AdditionalDownloads = config.AdditionalDownloads{Marquee: config.ArtKindMarquee, Video: true, Manual: true}
	e.srv.FailPath(rommAsset + "video.mp4")
	res := Art(context.Background(), e.input(d, c), nil)
	if !reflect.DeepEqual(res.Failed, []firmware.ArtKind{firmware.ArtVideo, firmware.ArtMarquee}) {
		t.Fatalf("failed %v", res.Failed)
	}
	if got := writtenKinds(res); !reflect.DeepEqual(got, []firmware.ArtKind{firmware.ArtCover, firmware.ArtManual}) {
		t.Fatalf("written %v", got)
	}
	if e.exists(t, path.Join(e.romDir, "images", "Game-marquee.png")) {
		t.Fatal("failed kind left a file")
	}
}

func TestArtOnlyMissing(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("NEWVIDEO")
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png"},
		SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"},
	})
	c := baseCfg()
	c.AdditionalDownloads.Video = true
	cover := path.Join(e.romDir, "images", "Game.png")
	realCover, _ := e.w.Real(cover)
	os.MkdirAll(filepath.Dir(realCover), 0755)
	os.WriteFile(realCover, []byte("OLDCOVER"), 0644)
	in := e.input(d, c)
	in.OnlyMissing = true
	var rec recorder
	res := Art(context.Background(), in, rec.add)
	if got := writtenKinds(res); !reflect.DeepEqual(got, []firmware.ArtKind{firmware.ArtVideo}) || len(res.Failed) != 0 {
		t.Fatalf("written %v failed %v", got, res.Failed)
	}
	if string(e.read(t, cover)) != "OLDCOVER" {
		t.Fatal("existing cover replaced")
	}
	if last := rec.events[len(rec.events)-1]; last.Total != 1 || last.Done != 1 {
		t.Fatalf("progress %+v", last)
	}
	for _, call := range e.srv.Calls() {
		if strings.Contains(call.Path, "cover") {
			t.Fatal("cover was fetched")
		}
	}
}

func TestArtReplacesExistingByDefault(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
	d := e.detail(t, romm.RomDetail{Rom: romm.Rom{PathCoverSmall: rommAsset + "cover/small.png"}})
	cover := path.Join(e.romDir, "images", "Game.png")
	realCover, _ := e.w.Real(cover)
	os.MkdirAll(filepath.Dir(realCover), 0755)
	os.WriteFile(realCover, []byte("OLDCOVER"), 0644)
	res := Art(context.Background(), e.input(d, baseCfg()), nil)
	if len(res.Written) != 1 || bytes.Equal(e.read(t, cover), []byte("OLDCOVER")) {
		t.Fatalf("res %+v", res)
	}
}

func TestArtNoCoverStillWritesOthers(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("V")
	d := e.detail(t, romm.RomDetail{SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"}})
	c := baseCfg()
	c.AdditionalDownloads.Video = true
	res := Art(context.Background(), e.input(d, c), nil)
	if got := writtenKinds(res); !reflect.DeepEqual(got, []firmware.ArtKind{firmware.ArtVideo}) || len(res.Failed) != 0 {
		t.Fatalf("written %v failed %v", got, res.Failed)
	}
}

func TestExternalArtSendsNoAuth(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.extData["/title.png"] = pngBytes(t, 12, 12)
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 12, 12)
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png"},
		SSMetadata: &romm.SSMetadata{TitleScreenURL: e.external.URL + "/title.png"},
	})
	c := baseCfg()
	c.AdditionalDownloads.Thumbnail = config.ArtKindTitle
	in := e.input(d, c)
	in.Client = romm.New(romm.Options{BaseURL: e.srv.URL(), AuthHeader: "Bearer secret"})
	res := Art(context.Background(), in, nil)
	if len(res.Failed) != 0 || len(res.Written) != 2 {
		t.Fatalf("res %+v", res)
	}
	e.extMu.Lock()
	defer e.extMu.Unlock()
	if len(e.extAuth) == 0 {
		t.Fatal("external server never hit")
	}
	for _, a := range e.extAuth {
		if a != "" {
			t.Fatalf("external request carried Authorization %q", a)
		}
	}
	var sawAuth bool
	for _, call := range e.srv.Calls() {
		if strings.HasPrefix(call.Path, "/assets/") && call.Auth == "Bearer secret" {
			sawAuth = true
		}
	}
	if !sawAuth {
		t.Fatal("RomM asset request must carry auth")
	}
}

func TestArtCancelFailsRemaining(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("V")
	e.srv.Assets[rommAsset+"manual.pdf"] = []byte("M")
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png", HasManual: true},
		PathManual: rommAsset + "manual.pdf",
		SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"},
	})
	c := baseCfg()
	c.AdditionalDownloads = config.AdditionalDownloads{Video: true, Manual: true}
	ctx, cancel := context.WithCancel(context.Background())
	in := e.input(d, c)
	res := Art(ctx, in, func(p Progress) {
		if p.Done == 1 {
			cancel()
		}
	})
	if got := writtenKinds(res); !reflect.DeepEqual(got, []firmware.ArtKind{firmware.ArtCover}) {
		t.Fatalf("written %v", got)
	}
	if !reflect.DeepEqual(res.Failed, []firmware.ArtKind{firmware.ArtVideo, firmware.ArtManual}) {
		t.Fatalf("failed %v", res.Failed)
	}
}

func TestArtSkipsKindsWithoutSource(t *testing.T) {
	e := newArtEnv(t, "knulli")
	d := e.detail(t, romm.RomDetail{})
	c := baseCfg()
	c.AdditionalDownloads.Video = true
	res := Art(context.Background(), e.input(d, c), nil)
	if len(res.Written) != 0 || len(res.Failed) != 0 {
		t.Fatalf("res %+v", res)
	}
}

func TestArtRejectsUnsafeStem(t *testing.T) {
	for _, stem := range []string{"", ".", "..", "a/b", `a\b`, ".grout-x"} {
		e := newArtEnv(t, "knulli")
		e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
		e.srv.Assets[rommAsset+"video.mp4"] = []byte("V")
		d := e.detail(t, romm.RomDetail{
			Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png"},
			SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"},
		})
		c := baseCfg()
		c.AdditionalDownloads.Video = true
		in := e.input(d, c)
		in.Stem = stem
		res := Art(context.Background(), in, nil)
		if len(res.Written) != 0 || !reflect.DeepEqual(res.Failed, []firmware.ArtKind{firmware.ArtCover, firmware.ArtVideo}) {
			t.Fatalf("stem %q: res %+v", stem, res)
		}
		for _, call := range e.srv.Calls() {
			if strings.HasPrefix(call.Path, "/assets/") {
				t.Fatalf("stem %q: asset fetched", stem)
			}
		}
	}
}

func TestArtRawOverCapFails(t *testing.T) {
	old := maxRawBytes
	maxRawBytes = 8
	defer func() { maxRawBytes = old }()
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("0123456789")
	e.srv.Assets[rommAsset+"manual.pdf"] = []byte("01234567")
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{HasManual: true},
		PathManual: rommAsset + "manual.pdf",
		SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"},
	})
	c := baseCfg()
	c.AdditionalDownloads = config.AdditionalDownloads{Video: true, Manual: true}
	res := Art(context.Background(), e.input(d, c), nil)
	if !reflect.DeepEqual(res.Failed, []firmware.ArtKind{firmware.ArtVideo}) || !reflect.DeepEqual(writtenKinds(res), []firmware.ArtKind{firmware.ArtManual}) {
		t.Fatalf("res %+v", res)
	}
	if e.exists(t, path.Join(e.romDir, "videos", "Game.mp4")) {
		t.Fatal("oversized video left a file")
	}
}

func TestArtRawOverCapWithoutLengthFails(t *testing.T) {
	old := maxRawBytes
	maxRawBytes = 8
	defer func() { maxRawBytes = old }()
	e := newArtEnv(t, "knulli")
	e.extData["/v.mp4"] = []byte("0123456789")
	d := e.detail(t, romm.RomDetail{SSMetadata: &romm.SSMetadata{VideoURL: e.external.URL + "/v.mp4"}})
	c := baseCfg()
	c.AdditionalDownloads.Video = true
	in := e.input(d, c)
	res := Art(context.Background(), in, nil)
	if !reflect.DeepEqual(res.Failed, []firmware.ArtKind{firmware.ArtVideo}) || len(res.Written) != 0 {
		t.Fatalf("res %+v", res)
	}
	realDir, _ := e.w.Real(path.Join(e.romDir, "videos"))
	entries, _ := os.ReadDir(realDir)
	if len(entries) != 0 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestArtSharedURLFetchedOnce(t *testing.T) {
	e := newArtEnv(t, "knulli")
	e.srv.Assets[rommAsset+"marquee.png"] = pngBytes(t, 10, 10)
	d := e.detail(t, romm.RomDetail{SSMetadata: &romm.SSMetadata{MarqueePath: rommAsset + "marquee.png"}})
	c := baseCfg()
	c.AdditionalDownloads = config.AdditionalDownloads{Marquee: config.ArtKindMarquee, Thumbnail: config.ArtKindMarquee}
	res := Art(context.Background(), e.input(d, c), nil)
	if len(res.Failed) != 0 || len(res.Written) != 2 {
		t.Fatalf("res %+v", res)
	}
	n := 0
	for _, call := range e.srv.Calls() {
		if call.Path == rommAsset+"marquee.png" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("fetched %d times", n)
	}
	a := e.read(t, path.Join(e.romDir, "images", "Game-marquee.png"))
	b := e.read(t, path.Join(e.romDir, "images", "Game-thumb.png"))
	if !bytes.Equal(a, b) {
		t.Fatal("files differ")
	}
}

func TestCapReaderFailsPastLimit(t *testing.T) {
	r := &capReader{r: strings.NewReader("0123456789"), left: 8}
	if _, err := io.ReadAll(r); !errors.Is(err, ErrArtTooLarge) {
		t.Fatalf("err=%v", err)
	}
	r = &capReader{r: strings.NewReader("01234567"), left: 8}
	if data, err := io.ReadAll(r); err != nil || len(data) != 8 {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestArtUnsafeStemSkipsKindsWithoutDir(t *testing.T) {
	e := newArtEnv(t, "nextui")
	e.srv.Assets[rommAsset+"cover/small.png"] = pngBytes(t, 10, 10)
	e.srv.Assets[rommAsset+"video.mp4"] = []byte("V")
	d := e.detail(t, romm.RomDetail{
		Rom:        romm.Rom{PathCoverSmall: rommAsset + "cover/small.png"},
		SSMetadata: &romm.SSMetadata{VideoPath: rommAsset + "video.mp4"},
	})
	c := baseCfg()
	c.AdditionalDownloads.Video = true
	in := e.input(d, c)
	in.Stem = ".."
	res := Art(context.Background(), in, nil)
	if len(res.Written) != 0 || !reflect.DeepEqual(res.Failed, []firmware.ArtKind{firmware.ArtCover}) {
		t.Fatalf("res %+v", res)
	}
}

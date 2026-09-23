package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"grout/cfw"
	"grout/library"

	uatomic "go.uber.org/atomic"

	"grout/romm"
	"grout/settings"
)

// Art for a game whose rom never arrived would leave a cover with nothing to
// go with it, which the frontend then lists as a phantom entry.
func TestFetchArt_SkipsGamesThatDidNotArrive(t *testing.T) {
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served.Add(1)
		_, _ = w.Write([]byte("art"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	items := []Item{
		{URL: srv.URL + "/mario.png", Location: filepath.Join(dir, "Mario.png"), GameName: "Mario"},
		{URL: srv.URL + "/zelda.png", Location: filepath.Join(dir, "Zelda.png"), GameName: "Zelda"},
	}

	fetcher := romm.NewArtFetcher(settings.Host{RootURI: srv.URL}, 0)
	progress := &uatomic.Float64{}

	// Only Mario's rom landed.
	FetchArt(fetcher, items, []romm.Rom{{Name: "Mario"}}, progress)

	if got := served.Load(); got != 1 {
		t.Errorf("fetched %d files, want only the one whose rom arrived", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "Mario.png")); err != nil {
		t.Errorf("Mario's art was not saved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Zelda.png")); !os.IsNotExist(err) {
		t.Error("Zelda's art was saved even though its rom never arrived")
	}
	if got := progress.Load(); got != 1 {
		t.Errorf("progress ended at %v, want 1", got)
	}
}

// Nothing to fetch must not leave the progress bar part way, and must not
// divide by a zero count.
func TestFetchArt_NothingWanted(t *testing.T) {
	progress := &uatomic.Float64{}
	fetcher := romm.NewArtFetcher(settings.Host{RootURI: "http://example.invalid"}, 0)

	FetchArt(fetcher, []Item{{GameName: "Mario"}}, nil, progress)

	if got := progress.Load(); got != 0 {
		t.Errorf("progress = %v, want it untouched when there was nothing to do", got)
	}
}

// One unreachable cover is not worth losing the rest of the run to.
func TestFetchArt_ContinuesPastAFailure(t *testing.T) {
	var served atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing.png" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		served.Add(1)
		_, _ = w.Write([]byte("art"))
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	items := []Item{
		{URL: srv.URL + "/missing.png", Location: filepath.Join(dir, "Missing.png"), GameName: "Mario"},
		{URL: srv.URL + "/ok.png", Location: filepath.Join(dir, "Ok.png"), GameName: "Mario"},
	}

	fetcher := romm.NewArtFetcher(settings.Host{RootURI: srv.URL}, 0)
	FetchArt(fetcher, items, []romm.Rom{{Name: "Mario"}}, &uatomic.Float64{})

	if got := served.Load(); got != 1 {
		t.Errorf("served %d, want the second cover fetched after the first failed", got)
	}
}

// The backfill and the download must agree on what a game's art is called. If
// one writes a name the other never looks for, every sync fetches it again.
func TestArtFor_AgreesWithThePlan(t *testing.T) {
	t.Setenv("BASE_PATH", t.TempDir())
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	config := settings.Config{
		DownloadArt:                  true,
		DownloadArtScreenshotPreview: true,
		DownloadSplashArt:            library.ArtKindScreenshot,
	}
	host := settings.Host{RootURI: "http://romm.local"}
	platform := romm.Platform{ID: 3, FSSlug: "snes", Name: "SNES"}
	game := romm.Rom{
		ID: 1, Name: "Mario", FsNameNoExt: "Mario",
		PathCoverLarge:    "/covers/1.png",
		MergedScreenshots: []string{"/screens/1.png"},
		Files:             []romm.RomFile{{ID: 1, FileName: "Mario.sfc"}},
	}

	plan, _ := BuildPlan(config, host, platform, []romm.Rom{game}, 0)
	direct := ArtFor(config, host, game, platform)

	planned := make(map[string]bool, len(plan.Art))
	for _, item := range plan.Art {
		planned[item.Location] = true
	}

	// More than one kind, or the comparison proves nothing.
	if len(direct) < 2 {
		t.Fatalf("ArtFor returned %d files, too few for this to be a real comparison", len(direct))
	}
	for _, item := range direct {
		if !planned[item.Location] {
			t.Errorf("ArtFor writes %q, which a download run never creates", item.Location)
		}
	}
	if len(direct) != len(plan.Art) {
		t.Errorf("ArtFor lists %d files, the plan %d: the two have drifted apart", len(direct), len(plan.Art))
	}
}

// A game the server has no cover for has no artwork to chase.
func TestArtFor_NoCover(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	got := ArtFor(settings.Config{}, settings.Host{}, romm.Rom{Name: "Homebrew"}, romm.Platform{FSSlug: "snes"})
	if got != nil {
		t.Errorf("ArtFor = %v, want nothing for a game with no cover", got)
	}
}

func TestMissing(t *testing.T) {
	dir := t.TempDir()
	here := filepath.Join(dir, "here.png")
	if err := os.WriteFile(here, []byte("art"), 0644); err != nil {
		t.Fatal(err)
	}

	got := Missing([]Item{
		{Location: here},
		{Location: filepath.Join(dir, "gone.png")},
	})

	if len(got) != 1 || filepath.Base(got[0].Location) != "gone.png" {
		t.Errorf("Missing = %v, want only the absent file", got)
	}
}

// A backfill of artwork should not pull down a video per game across a whole
// library. Those arrive with a game instead.
func TestImages_DropsVideosAndManuals(t *testing.T) {
	got := Images([]Item{
		{Location: "cover.png", IsImage: true},
		{Location: "trailer.mp4"},
		{Location: "manual.pdf"},
		{Location: "marquee.png", IsImage: true},
	})

	if len(got) != 2 {
		t.Fatalf("Images kept %d items, want the 2 images", len(got))
	}
	for _, item := range got {
		if !item.IsImage {
			t.Errorf("%q is not an image", item.Location)
		}
	}
}

package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

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

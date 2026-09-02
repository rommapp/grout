package romm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// newTestArtFetcher returns a Fetcher pointed at srv with image processing
// stubbed, since the real one needs an SDL window.
func newTestArtFetcher(srv *httptest.Server, process processFunc) *ArtFetcher {
	f := NewArtFetcher(Host{}, 0)
	f.client = srv.Client()
	if process != nil {
		f.Process = process
	} else {
		f.Process = func(string) error { return nil }
	}
	return f
}

func TestFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	got, err := newTestArtFetcher(srv, nil).Fetch(srv.URL + "/cover.png")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(got) != "image-bytes" {
		t.Errorf("Fetch = %q, want %q", got, "image-bytes")
	}
}

func TestFetch_SendsAuthorizationHeader(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	f := newTestArtFetcher(srv, nil)
	f.host = Host{Token: "test-token"}
	if _, err := f.Fetch(srv.URL); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if seen != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", seen, "Bearer test-token")
	}
}

// RomM serves art under paths containing the rom name, which usually has
// spaces.
func TestFetch_EscapesSpacesInTheURL(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
	}))
	defer srv.Close()

	if _, err := newTestArtFetcher(srv, nil).Fetch(srv.URL + "/Sonic the Hedgehog.png"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if seenPath != "/Sonic the Hedgehog.png" {
		t.Errorf("server saw path %q, want the spaces to survive as %%20", seenPath)
	}
}

func TestFetch_ErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := newTestArtFetcher(srv, nil).Fetch(srv.URL); err == nil {
		t.Fatal("expected an error for a 404")
	}
}

func TestSave_WritesTheFileAndCreatesParents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "media", "images", "Sonic.png")
	if err := newTestArtFetcher(srv, nil).Save(srv.URL, dest); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(got) != "png-bytes" {
		t.Errorf("file contents = %q, want %q", got, "png-bytes")
	}
}

// An error response must not survive on disk as artwork. Leaving it there is
// worse than having nothing: the file exists, so the next sync sees the art as
// present and never retries.
func TestSave_LeavesNoFileOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("<html>error</html>"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "Sonic.png")
	if err := newTestArtFetcher(srv, nil).Save(srv.URL, dest); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("expected no file at %s, stat err = %v", dest, err)
	}
}

// A 200 response that is not an image must be removed too, otherwise a proxy
// login page gets written where a cover should be.
func TestSave_RemovesTheFileWhenItIsNotAnImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "Sonic.png")
	f := newTestArtFetcher(srv, func(string) error { return errors.New("not an image") })

	if err := f.Save(srv.URL, dest); err == nil {
		t.Fatal("expected an error when the payload is not an image")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("expected the file to be removed, stat err = %v", err)
	}
}

// Manuals and videos are downloaded as-is.
func TestSaveRaw_SkipsImageProcessing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("%PDF-1.4"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "manual.pdf")
	f := newTestArtFetcher(srv, func(string) error {
		t.Error("image processing must not run for SaveRaw")
		return nil
	})

	if err := f.SaveRaw(srv.URL, dest); err != nil {
		t.Fatalf("SaveRaw: %v", err)
	}
	if got, _ := os.ReadFile(dest); string(got) != "%PDF-1.4" {
		t.Errorf("file contents = %q", got)
	}
}

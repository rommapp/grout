package romm

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// processFunc normalises a downloaded image in place.
type processFunc func(path string) error

// ArtFetcher downloads artwork from one RomM host.
//
// It exists because four call sites each built their own request, auth header,
// status check and write loop, and drifted apart: only one honoured the host's
// self-signed certificate setting, only one checked that what came back was an
// image, and they disagreed about cleaning up a half-written file.
type ArtFetcher struct {
	client *http.Client
	host   Host

	// Process, when set, normalises a saved image in place. Save removes the
	// file if it returns an error, so a response that is not an image never
	// survives on disk. It is a field rather than a dependency because
	// resizing artwork needs the display, which this layer must not know
	// about.
	Process processFunc
}

// NewArtFetcher returns an ArtFetcher for host. A zero timeout uses the client
// default.
func NewArtFetcher(host Host, timeout time.Duration) *ArtFetcher {
	return &ArtFetcher{client: NewHTTPClient(host, timeout), host: host}
}

// Fetch returns the bytes at url.
func (f *ArtFetcher) Fetch(url string) ([]byte, error) {
	resp, err := f.get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read artwork body: %w", err)
	}
	return data, nil
}

// Save writes the image at url to dest and normalises it for display.
//
// dest is removed if anything fails, so an error response or a truncated
// transfer never survives as a file where artwork is expected. Callers that
// want the bytes without touching disk should use Fetch.
func (f *ArtFetcher) Save(url, dest string) error {
	return f.save(url, dest, true)
}

// SaveRaw writes url to dest without image processing, for artwork that is not
// an image -- manuals and videos.
func (f *ArtFetcher) SaveRaw(url, dest string) error {
	return f.save(url, dest, false)
}

func (f *ArtFetcher) save(url, dest string, isImage bool) error {
	resp, err := f.get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("create artwork directory: %w", err)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create artwork file: %w", err)
	}

	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(dest)
		return fmt.Errorf("write artwork file: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(dest)
		return fmt.Errorf("close artwork file: %w", closeErr)
	}

	if isImage && f.Process != nil {
		if err := f.Process(dest); err != nil {
			os.Remove(dest)
			return fmt.Errorf("process artwork image: %w", err)
		}
	}

	return nil
}

func (f *ArtFetcher) get(url string) (*http.Response, error) {
	// RomM serves art under paths containing the rom's name, which routinely
	// has spaces in it.
	req, err := http.NewRequest(http.MethodGet, strings.ReplaceAll(url, " ", "%20"), nil)
	if err != nil {
		return nil, fmt.Errorf("build artwork request: %w", err)
	}
	req.Header.Set("Authorization", f.host.AuthHeader())

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch artwork: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("fetch artwork: unexpected status %s", resp.Status)
	}
	return resp, nil
}

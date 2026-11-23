package romm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDownloadStreamsWithSize(t *testing.T) {
	var auth, query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		query = r.URL.RawQuery
		w.Header().Set("Content-Length", "5")
		w.Write([]byte("hello"))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, AuthHeader: "Bearer t"})
	body, err := c.Download(context.Background(), RomContentPath(1, "a.gb"), url.Values{"purpose": {"download"}})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "hello" || body.Size != 5 || auth != "Bearer t" || query != "purpose=download" {
		t.Fatalf("data=%q size=%d auth=%q query=%q", data, body.Size, auth, query)
	}
}

func TestDownloadUnknownSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		w.Write([]byte("x"))
	}))
	defer srv.Close()
	body, err := New(Options{BaseURL: srv.URL}).Download(context.Background(), "/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if body.Size != -1 {
		t.Fatalf("size=%d", body.Size)
	}
}

func TestDownloadStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()
	_, err := New(Options{BaseURL: srv.URL}).Download(context.Background(), "/missing", nil)
	if !IsNotFound(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestContentTimeoutIgnoresAPITimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("slow"))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Timeout: 10 * time.Millisecond})
	if _, err := c.Download(context.Background(), "/x", nil); err == nil {
		t.Fatal("api client should time out")
	}
	body, err := c.WithContentTimeout(time.Second).Download(context.Background(), "/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "slow" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}

func TestContentTimeoutDeadlineApplies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Write([]byte("slow"))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL}).WithContentTimeout(20 * time.Millisecond)
	if _, err := c.Download(context.Background(), "/x", nil); err == nil {
		t.Fatal("deadline should apply")
	}
}

func TestContentTimeoutZeroHasNoDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	c := New(Options{BaseURL: srv.URL, Timeout: 10 * time.Millisecond}).WithContentTimeout(0)
	body, err := c.Download(context.Background(), "/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
}

func TestRomContentPathEscapes(t *testing.T) {
	name := "Final Fantasy VII (Disc 1).chd"
	want := "/api/roms/7/content/" + url.PathEscape(name)
	if got := RomContentPath(7, name); got != want || !strings.Contains(got, "%20") {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := FirmwareContentPath(3, "bios 1.bin"); got != "/api/firmware/3/content/bios%201.bin" {
		t.Fatalf("got %q", got)
	}
}

func TestAssetURL(t *testing.T) {
	c := New(Options{BaseURL: "http://romm.local/"})
	cases := map[string]string{
		"https://cdn.example/a b.png":                                       "https://cdn.example/a%20b.png",
		"/assets/romm/resources/roms/1/my cover.png?ts=2026-01-01 00:00:00": "http://romm.local/assets/romm/resources/roms/1/my%20cover.png?ts=2026-01-01%2000:00:00",
		"http://cdn.example/a.png":                                          "http://cdn.example/a.png",
		"roms/1/cover/small.png?ts=1":                                       "http://romm.local/assets/romm/resources/roms/1/cover/small.png?ts=1",
		"/roms/1/cover/a b.png":                                             "http://romm.local/assets/romm/resources/roms/1/cover/a%20b.png",
		"/assets/romm/resources/roms/1/c.png":                               "http://romm.local/assets/romm/resources/roms/1/c.png",
		"":                                                                  "",
	}
	for in, want := range cases {
		if got := c.AssetURL(in); got != want {
			t.Errorf("AssetURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDownloadURLAuthOnlyOnBaseHost(t *testing.T) {
	var mu sync.Mutex
	var sameAuth, otherAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		otherAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Write([]byte("ext"))
	}))
	defer other.Close()
	base := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sameAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Write([]byte("own"))
	}))
	defer base.Close()
	c := New(Options{BaseURL: base.URL, AuthHeader: "Bearer t"})
	for _, tc := range []struct{ url, want string }{{base.URL + "/a.png", "own"}, {other.URL + "/b.png", "ext"}} {
		body, err := c.DownloadURL(context.Background(), tc.url)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(body)
		body.Close()
		if string(data) != tc.want {
			t.Fatalf("data=%q want %q", data, tc.want)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if sameAuth != "Bearer t" || otherAuth != "" {
		t.Fatalf("same=%q other=%q", sameAuth, otherAuth)
	}
}

func TestDownloadURLRedirectToOtherHostDropsAuth(t *testing.T) {
	var mu sync.Mutex
	var otherAuth string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		otherAuth = r.Header.Get("Authorization")
		mu.Unlock()
		w.Write([]byte("ok"))
	}))
	defer other.Close()
	base := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/x", http.StatusFound)
	}))
	defer base.Close()
	c := New(Options{BaseURL: base.URL, AuthHeader: "Bearer t"})
	body, err := c.DownloadURL(context.Background(), base.URL+"/r")
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	mu.Lock()
	defer mu.Unlock()
	if otherAuth != "" {
		t.Fatalf("auth leaked: %q", otherAuth)
	}
}

func TestDownloadURLRefusesOtherSchemes(t *testing.T) {
	c := New(Options{BaseURL: "http://127.0.0.1:1", AuthHeader: "Bearer t"})
	for _, u := range []string{"file:///etc/passwd", "ftp://x/y", "gopher://x", "/relative", ""} {
		if _, err := c.DownloadURL(context.Background(), u); err == nil {
			t.Fatalf("%q accepted", u)
		}
	}
}

func TestDownloadURLStatusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }))
	defer srv.Close()
	_, err := New(Options{BaseURL: srv.URL}).DownloadURL(context.Background(), srv.URL+"/m")
	if !IsNotFound(err) {
		t.Fatalf("err=%v", err)
	}
}

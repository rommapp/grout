package romm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rommapp/grout/core/config"
)

const detailFixture = `{
  "id": 12,
  "platform_id": 3,
  "name": "Zelda",
  "has_manual": true,
  "path_cover_small": "roms/3/12/cover/small.png",
  "path_cover_large": "roms/3/12/cover/big.png",
  "url_cover": "https://igdb.example/c.png",
  "path_manual": "roms/3/12/manual/12.pdf",
  "url_manual": "https://igdb.example/m.pdf",
  "path_screenshots": ["roms/3/12/screenshots/0.jpg"],
  "merged_screenshots": ["/assets/romm/resources/roms/3/12/screenshots/m.jpg"],
  "user_screenshots": [{"url_path": "/assets/romm/resources/user/s.png"}],
  "ss_metadata": {
    "box2d_url": "https://ss.example/box2d.png",
    "box3d_path": "roms/3/12/box3d.png",
    "box3d_url": "https://ss.example/box3d.png",
    "miximage_url": "https://ss.example/mix.png",
    "miximage_v2_path": "roms/3/12/mixv2.png",
    "title_screen_url": "https://ss.example/title.png",
    "screenshot_url": "https://ss.example/ss.png",
    "marquee_path": "roms/3/12/marquee.png",
    "logo_url": "https://ss.example/logo.png",
    "bezel_path": "roms/3/12/bezel.png",
    "manual_url": "https://ss.example/manual.pdf",
    "box2d_back_path": "roms/3/12/back.png",
    "fanart_url": "https://ss.example/fan.png",
    "video_path": "roms/3/12/video.mp4"
  }
}`

func TestRomDetailDecodes(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(detailFixture))
	}))
	defer srv.Close()
	d, err := New(Options{BaseURL: srv.URL}).RomDetail(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/api/roms/12" || d.ID != 12 || d.Name != "Zelda" || !d.HasManual {
		t.Fatalf("path=%q d=%+v", path, d.Rom)
	}
	if d.PathManual != "roms/3/12/manual/12.pdf" || d.URLManual != "https://igdb.example/m.pdf" {
		t.Fatalf("manual %q %q", d.PathManual, d.URLManual)
	}
	if len(d.PathScreenshots) != 1 || len(d.MergedScreenshots) != 1 || len(d.UserScreenshots) != 1 {
		t.Fatalf("screenshots %+v", d)
	}
	if d.SSMetadata == nil || d.SSMetadata.Box3DPath != "roms/3/12/box3d.png" || d.SSMetadata.VideoPath != "roms/3/12/video.mp4" || d.SSMetadata.TitleScreenURL != "https://ss.example/title.png" {
		t.Fatalf("ss = %+v", d.SSMetadata)
	}
}

func TestRomDetailNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(http.NotFound))
	defer srv.Close()
	_, err := New(Options{BaseURL: srv.URL}).RomDetail(context.Background(), 1)
	if !IsNotFound(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestArtSourceMatchesV2(t *testing.T) {
	c := New(Options{BaseURL: "http://h"})
	const res = "http://h/assets/romm/resources/"
	var full RomDetail
	if err := json.Unmarshal([]byte(detailFixture), &full); err != nil {
		t.Fatal(err)
	}
	full.client = c

	noSS := full
	noSS.SSMetadata = nil

	onlyLarge := RomDetail{Rom: Rom{PathCoverLarge: "big.png"}, client: c}
	onlyURL := RomDetail{Rom: Rom{URLCover: "https://x/c d.png"}, client: c}
	noManualFlag := full
	noManualFlag.HasManual = false
	noManualPath := full
	noManualPath.PathManual = ""
	noManualPath.URLManual = ""
	noUser := full
	noUser.UserScreenshots = nil
	noUserMerged := noUser
	noUserMerged.MergedScreenshots = nil
	empty := RomDetail{client: c}

	cases := []struct {
		name string
		d    RomDetail
		src  ArtSource
		kind config.ArtKind
		want string
	}{
		{"cover default", full, SourceCover, config.ArtKindDefault, res + "roms/3/12/cover/small.png"},
		{"cover none kind", full, SourceCover, config.ArtKindNone, res + "roms/3/12/cover/small.png"},
		{"cover large fallback", onlyLarge, SourceCover, config.ArtKindDefault, res + "big.png"},
		{"cover url fallback escaped", onlyURL, SourceCover, config.ArtKindDefault, "https://x/c%20d.png"},
		{"box2d url", full, SourceCover, config.ArtKindBox2D, "https://ss.example/box2d.png"},
		{"box2d falls to cover", noSS, SourceCover, config.ArtKindBox2D, res + "roms/3/12/cover/small.png"},
		{"box3d path wins", full, SourceCover, config.ArtKindBox3D, res + "roms/3/12/box3d.png"},
		{"miximage url", full, SourceCover, config.ArtKindMixImage, "https://ss.example/mix.png"},
		{"miximage v2 path", full, SourceCover, config.ArtKindMixImageV2, res + "roms/3/12/mixv2.png"},
		{"empty cover", empty, SourceCover, config.ArtKindDefault, ""},
		{"screenshot user first", full, SourceScreenshot, config.ArtKindNone, "http://h/assets/romm/resources/user/s.png"},
		{"screenshot merged", noUser, SourceScreenshot, config.ArtKindNone, "http://h/assets/romm/resources/roms/3/12/screenshots/m.jpg"},
		{"screenshot ss", noUserMerged, SourceScreenshot, config.ArtKindNone, "https://ss.example/ss.png"},
		{"screenshot none", empty, SourceScreenshot, config.ArtKindNone, ""},
		{"splash marquee", full, SourceSplash, config.ArtKindMarquee, res + "roms/3/12/marquee.png"},
		{"splash title", full, SourceSplash, config.ArtKindTitle, "https://ss.example/title.png"},
		{"splash other kind", full, SourceSplash, config.ArtKindLogo, ""},
		{"marquee kind marquee", full, SourceMarquee, config.ArtKindMarquee, res + "roms/3/12/marquee.png"},
		{"marquee kind logo", full, SourceMarquee, config.ArtKindLogo, "https://ss.example/logo.png"},
		{"marquee other kind", full, SourceMarquee, config.ArtKindTitle, ""},
		{"video path", full, SourceVideo, config.ArtKindVideo, res + "roms/3/12/video.mp4"},
		{"bezel path", full, SourceBezel, config.ArtKindNone, res + "roms/3/12/bezel.png"},
		{"manual rom path", full, SourceManual, config.ArtKindNone, res + "roms/3/12/manual/12.pdf"},
		{"manual no flag uses ss", noManualFlag, SourceManual, config.ArtKindNone, "https://ss.example/manual.pdf"},
		{"manual flag url only", func() RomDetail { d := noManualPath; d.HasManual = true; return d }(), SourceManual, config.ArtKindNone, "https://ss.example/manual.pdf"},
		{"manual url fallback", func() RomDetail { d := full; d.PathManual = ""; return d }(), SourceManual, config.ArtKindNone, "https://igdb.example/m.pdf"},
		{"boxback path", full, SourceBoxback, config.ArtKindNone, res + "roms/3/12/back.png"},
		{"fanart url", full, SourceFanart, config.ArtKindNone, "https://ss.example/fan.png"},
		{"unknown source", full, ArtSource("nope"), config.ArtKindNone, ""},
		{"no ss video", noSS, SourceVideo, config.ArtKindVideo, ""},
	}
	for _, tc := range cases {
		if got := tc.d.ArtSource(tc.src, tc.kind); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

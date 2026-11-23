package grout

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/test/fakeromm"
)

func ptr[T any](v T) *T {
	return &v
}

func TestUpdateSettingsAppliesOnlySetFields(t *testing.T) {
	ctx := context.Background()
	srv := gbaServer(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	srv.Collections[romm.CollectionRegular] = []fakeromm.Collection{{ID: "7", Name: "Favourites", RomIDs: []int{1, 2}, UpdatedAt: base}}
	g := connected(t, srv)
	if _, err := g.UpdateSettings(SettingsPatch{PlatformOrder: ptr([]string{"gba"})}); err != nil {
		t.Fatal(err)
	}

	before := g.Settings()
	if before.ShowRegularCollections {
		t.Fatal("regular collections shown by default")
	}
	got, err := g.UpdateSettings(SettingsPatch{ShowRegularCollections: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	want := before
	want.ShowRegularCollections = true
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("returned = %+v, want %+v", got, want)
	}
	if after := g.Settings(); !reflect.DeepEqual(after, want) {
		t.Fatalf("settings = %+v, want %+v", after, want)
	}

	if _, err := g.Refresh(ctx, RefreshOptions{}, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := g.Collections(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Name != "Favourites" || rows[0].Games != 2 {
		t.Fatalf("collections = %+v", rows)
	}
}

func TestUpdateSettingsRejectsBadValues(t *testing.T) {
	g := openTest(t)
	before := g.Settings()
	for name, p := range map[string]SettingsPatch{
		"downloaded games":      {DownloadedGames: ptr(config.DownloadedGamesMode("hide"))},
		"log level":             {LogLevel: ptr(config.LogLevel("TRACE"))},
		"api timeout zero":      {APITimeout: ptr(time.Duration(0))},
		"api timeout negative":  {APITimeout: ptr(-time.Second)},
		"api timeout too long":  {APITimeout: ptr(301 * time.Second)},
		"download timeout zero": {DownloadTimeout: ptr(time.Duration(0))},
		"download timeout long": {DownloadTimeout: ptr(24*time.Hour + time.Second)},
		"duplicate platform":    {ShowSmartCollections: ptr(true), PlatformOrder: ptr([]string{"gba", "snes", "gba"})},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := g.UpdateSettings(p)
			wantCode(t, err, CodeBadRequest)
			if after := g.Settings(); !reflect.DeepEqual(after, before) {
				t.Fatalf("settings changed: %+v", after)
			}
		})
	}

	got, err := g.UpdateSettings(SettingsPatch{
		APITimeout:      ptr(300 * time.Second),
		DownloadTimeout: ptr(24 * time.Hour),
		DownloadedGames: ptr(config.DownloadedGamesModeFilter),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ApiTimeout.Duration() != 300*time.Second || got.DownloadTimeout.Duration() != 24*time.Hour || got.DownloadedGames != config.DownloadedGamesModeFilter {
		t.Fatalf("settings = %+v", got)
	}
}

func TestUpdateSettingsLogLevelTakesEffect(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	g := open(t, testOptions(t, dataDir))
	logPath := filepath.Join(dataDir, "logs", "grout.log")
	const line = "checking server status"

	_, _ = g.Status(ctx)
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), line) {
		t.Fatalf("debug line logged at the default level:\n%s", b)
	}

	if _, err := g.UpdateSettings(SettingsPatch{LogLevel: ptr(config.LogLevelDebug)}); err != nil {
		t.Fatal(err)
	}
	_, _ = g.Status(ctx)
	b, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), line) {
		t.Fatalf("debug line missing after switching to DEBUG:\n%s", b)
	}
}

func TestUpdateSettingsLogLevelMatchesSavedUnderConcurrency(t *testing.T) {
	g := open(t, testOptions(t, t.TempDir()))
	levels := []config.LogLevel{config.LogLevelDebug, config.LogLevelError}
	for round := 0; round < 20; round++ {
		var wg sync.WaitGroup
		for _, l := range levels {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 10; i++ {
					if _, err := g.UpdateSettings(SettingsPatch{LogLevel: ptr(l)}); err != nil {
						t.Error(err)
						return
					}
				}
			}()
		}
		wg.Wait()
		if got, want := g.level.Level(), g.Settings().LogLevel.Slog(); got != want {
			t.Fatalf("round %d: live level %v, saved %v", round, got, want)
		}
	}
}

func TestUpdateSettingsArtFields(t *testing.T) {
	g := openTest(t)
	got, err := g.UpdateSettings(SettingsPatch{
		DownloadArt:                  ptr(true),
		UnzipDownloads:               ptr(true),
		ArtKind:                      ptr(config.ArtKindMixImageV2),
		DownloadArtScreenshotPreview: ptr(true),
		DownloadSplashArt:            ptr(config.ArtKindTitle),
		AdditionalDownloads: &AdditionalDownloadsPatch{
			Marquee:   ptr(config.ArtKindLogo),
			Thumbnail: ptr(config.ArtKindMarquee),
			Video:     ptr(true),
			Bezel:     ptr(true),
			Manual:    ptr(true),
			BoxBack:   ptr(true),
			Fanart:    ptr(true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := config.AdditionalDownloads{Marquee: config.ArtKindLogo, Thumbnail: config.ArtKindMarquee, Video: true, Bezel: true, Manual: true, BoxBack: true, Fanart: true}
	if !got.DownloadArt || !got.UnzipDownloads || got.ArtKind != config.ArtKindMixImageV2 || !got.DownloadArtScreenshotPreview ||
		got.DownloadSplashArt != config.ArtKindTitle || got.AdditionalDownloads != want {
		t.Fatalf("settings = %+v", got)
	}
	if after := g.Settings(); !reflect.DeepEqual(after, got) {
		t.Fatalf("saved = %+v", after)
	}
	for _, k := range config.CoverArtKinds() {
		if _, err := g.UpdateSettings(SettingsPatch{ArtKind: ptr(k)}); err != nil {
			t.Fatalf("art kind %q: %v", k, err)
		}
	}
	for _, k := range config.SplashArtKinds() {
		if _, err := g.UpdateSettings(SettingsPatch{DownloadSplashArt: ptr(k), AdditionalDownloads: &AdditionalDownloadsPatch{Thumbnail: ptr(k)}}); err != nil {
			t.Fatalf("splash kind %q: %v", k, err)
		}
	}
	for _, k := range config.MarqueeArtKinds() {
		if _, err := g.UpdateSettings(SettingsPatch{AdditionalDownloads: &AdditionalDownloadsPatch{Marquee: ptr(k)}}); err != nil {
			t.Fatalf("marquee kind %q: %v", k, err)
		}
	}

	before := g.Settings()
	for name, p := range map[string]SettingsPatch{
		"art kind none":       {ArtKind: ptr(config.ArtKindNone)},
		"art kind marquee":    {ArtKind: ptr(config.ArtKindMarquee)},
		"art kind unknown":    {ArtKind: ptr(config.ArtKind("Poster"))},
		"splash box2d":        {DownloadSplashArt: ptr(config.ArtKindBox2D)},
		"splash logo":         {DownloadSplashArt: ptr(config.ArtKindLogo)},
		"thumbnail logo":      {AdditionalDownloads: &AdditionalDownloadsPatch{Thumbnail: ptr(config.ArtKindLogo)}},
		"thumbnail unknown":   {AdditionalDownloads: &AdditionalDownloadsPatch{Thumbnail: ptr(config.ArtKind("Poster"))}},
		"marquee title":       {AdditionalDownloads: &AdditionalDownloadsPatch{Marquee: ptr(config.ArtKindTitle)}},
		"marquee default":     {AdditionalDownloads: &AdditionalDownloadsPatch{Marquee: ptr(config.ArtKindDefault)}},
		"valid with bad mate": {DownloadArt: ptr(false), ArtKind: ptr(config.ArtKindVideo)},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := g.UpdateSettings(p)
			wantCode(t, err, CodeBadRequest)
			if after := g.Settings(); !reflect.DeepEqual(after, before) {
				t.Fatalf("settings changed: %+v", after)
			}
		})
	}
}

func TestAdditionalDownloadsPatchIsPartial(t *testing.T) {
	g := openTest(t)
	start := config.AdditionalDownloads{Marquee: config.ArtKindMarquee, Thumbnail: config.ArtKindTitle, Video: true, Manual: true}
	if _, err := g.UpdateSettings(SettingsPatch{AdditionalDownloads: &AdditionalDownloadsPatch{
		Marquee: ptr(start.Marquee), Thumbnail: ptr(start.Thumbnail), Video: ptr(true), Manual: ptr(true),
	}}); err != nil {
		t.Fatal(err)
	}
	got, err := g.UpdateSettings(SettingsPatch{AdditionalDownloads: &AdditionalDownloadsPatch{Video: ptr(false), Fanart: ptr(true)}})
	if err != nil {
		t.Fatal(err)
	}
	want := start
	want.Video = false
	want.Fanart = true
	if got.AdditionalDownloads != want {
		t.Fatalf("additional = %+v, want %+v", got.AdditionalDownloads, want)
	}
	got, err = g.UpdateSettings(SettingsPatch{AdditionalDownloads: &AdditionalDownloadsPatch{Marquee: ptr(config.ArtKindNone)}})
	if err != nil {
		t.Fatal(err)
	}
	want.Marquee = config.ArtKindNone
	if got.AdditionalDownloads != want {
		t.Fatalf("additional = %+v, want %+v", got.AdditionalDownloads, want)
	}
	got, err = g.UpdateSettings(SettingsPatch{ShowSmartCollections: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got.AdditionalDownloads != want {
		t.Fatalf("untouched additional = %+v, want %+v", got.AdditionalDownloads, want)
	}
}

func TestUpdateSettingsGamelistOmitsRegion(t *testing.T) {
	ctx := context.Background()
	srv := fetchServer(t)
	srv.Roms[0].Regions = []string{"USA"}
	e := newFetchEnvWith(t, "knulli", srv, rootedHost(t))
	if e.g.Settings().GamelistOmitsRegion {
		t.Fatal("region omitted by default")
	}
	if _, err := e.g.FetchGame(ctx, 1, 0, nil); err != nil {
		t.Fatal(err)
	}
	file := e.gamelistPath(t)
	if body := string(e.read(t, file)); !strings.Contains(body, "<name>game 1 (USA)</name>") {
		t.Fatalf("gamelist.xml = %s", body)
	}
	got, err := e.g.UpdateSettings(SettingsPatch{GamelistOmitsRegion: ptr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if !got.GamelistOmitsRegion || !e.g.Settings().GamelistOmitsRegion {
		t.Fatalf("settings = %+v", got)
	}
	if _, err := e.g.GamelistRefresh(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if body := string(e.read(t, file)); !strings.Contains(body, "<name>game 1</name>") {
		t.Fatalf("gamelist.xml = %s", body)
	}
	if _, err := e.g.UpdateSettings(SettingsPatch{ShowSmartCollections: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if !e.g.Settings().GamelistOmitsRegion {
		t.Fatal("unrelated patch reset the setting")
	}
}

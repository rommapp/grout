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

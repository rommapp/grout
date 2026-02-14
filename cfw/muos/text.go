package muos

import (
	"fmt"
	"grout/internal/gamelist"
	"grout/internal/stringutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

func AddGameDescription(entry gamelist.RomGameEntry) {
	logger := gaba.GetLogger()
	textDir := GetTextDirectory(entry.Platform.FSSlug, entry.Platform.Name)
	if err := os.MkdirAll(textDir, 0755); err != nil {
		logger.Warn("Cannot create text directory", "path", textDir, "error", err)
		return
	}

	gameTextPath := filepath.Join(textDir, fmt.Sprintf("%s.txt", entry.Game.FsNameNoExt))
	gameTextFile, err := os.Create(gameTextPath)
	if err != nil {
		logger.Warn("Cannot create file", "path", gameTextPath, "error", err)
		return
	}
	defer gameTextFile.Close()

	var description strings.Builder
	description.WriteString(fmt.Sprintf("%s: %s\n", i18n.Localize(&goi18n.Message{ID: "game_details_name", Other: "Name"}, nil), stringutil.PrepareRomName(entry.Game.Name, entry.Game.Regions)))
	if entry.Game.Metadatum.FirstReleaseDate != 0 {
		t := time.Unix(entry.Game.Metadatum.FirstReleaseDate/1000, 0).UTC()
		formatted := t.Format("2006-01-02")
		description.WriteString(fmt.Sprintf("%s: %s\n", i18n.Localize(&goi18n.Message{ID: "game_details_release_date", Other: "Release Date"}, nil), formatted))
	}
	if len(entry.Game.Languages) > 0 {
		description.WriteString(fmt.Sprintf("%s: %s\n", i18n.Localize(&goi18n.Message{ID: "game_details_languages", Other: "Languages"}, nil), strings.Join(entry.Game.Languages, ", ")))
	}
	if len(entry.Game.Metadatum.Genres) > 0 {
		description.WriteString(fmt.Sprintf("%s: %s\n", i18n.Localize(&goi18n.Message{ID: "game_details_genres", Other: "Genres"}, nil), strings.Join(entry.Game.Metadatum.Genres, ", ")))
	}

	description.WriteString(fmt.Sprintf("\n%s: %s\n", i18n.Localize(&goi18n.Message{ID: "game_details_description", Other: "Description"}, nil), entry.Game.Summary))
	_, err = fmt.Fprint(gameTextFile, description.String())
	if err != nil {
		logger.Warn("Cannot write to file", "file", gameTextFile.Name(), "error", err)
	}
}

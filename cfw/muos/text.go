package muos

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/internal/gamelist"

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

	game := entry.Game
	gameTextPath := filepath.Join(textDir, fmt.Sprintf("%s.txt", game.BaseName))
	gameTextFile, err := os.Create(gameTextPath)
	if err != nil {
		logger.Warn("Cannot create file", "path", gameTextPath, "error", err)
		return
	}
	defer gameTextFile.Close()

	var description strings.Builder
	line := func(id, fallback, value string) {
		if value == "" {
			return
		}
		description.WriteString(fmt.Sprintf("%s: %s\n",
			i18n.Localize(&goi18n.Message{ID: id, Other: fallback}, nil), value))
	}

	line("game_details_name", "Name", game.DisplayName)
	if game.HasReleaseDate() {
		line("game_details_release_date", "Release Date", game.ReleaseDate.Format("2006-01-02"))
	}
	line("game_details_languages", "Languages", strings.Join(game.Languages, ", "))
	line("game_details_genres", "Genres", strings.Join(game.Genres, ", "))

	description.WriteString(fmt.Sprintf("\n%s: %s\n",
		i18n.Localize(&goi18n.Message{ID: "game_details_description", Other: "Description"}, nil), game.Summary))

	if _, err = fmt.Fprint(gameTextFile, description.String()); err != nil {
		logger.Warn("Cannot write to file", "file", gameTextFile.Name(), "error", err)
	}
}

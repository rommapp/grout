package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"grout/cache"
	"grout/catalog"
	"grout/romm"
	"grout/settings"
	"grout/textmatch"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"go.uber.org/atomic"
)

type GameDetailsInput struct {
	Config   *settings.Config
	Host     settings.Host
	Platform romm.Platform
	Game     romm.Rom
}

type GameDetailsOutput struct {
	Action            GameDetailsAction
	DownloadRequested bool
	SelectedFileID    int
	Game              romm.Rom
	Platform          romm.Platform
}

type GameDetailsScreen struct{}

func NewGameDetailsScreen() *GameDetailsScreen {
	return &GameDetailsScreen{}
}

// fileVersionDropdown identifies the version picker among the screen's
// sections, both when reading the result and when reacting to a change.
const fileVersionDropdown = "file_version"

func (s *GameDetailsScreen) Draw(input GameDetailsInput) (GameDetailsOutput, error) {
	output := GameDetailsOutput{
		Action:   GameDetailsActionBack,
		Game:     input.Game,
		Platform: input.Platform,
	}

	// A game shipping several versions is downloaded one version at a time, so
	// the picker takes the A button and downloading moves to X.
	versions := input.Game.Files
	picksVersion := input.Game.HasNestedSingleFile && len(versions) > 1

	sections := s.sections(input, picksVersion)

	downloadText := downloadLabel(catalog.IsDownloaded(*input.Config, input.Game))
	var dynamicText *atomic.String
	if picksVersion {
		// The picker opens on the first version, so the footer answers for
		// that one rather than for the game as a whole.
		downloadText = downloadLabel(catalog.IsFileDownloaded(*input.Config, input.Game, versions[0].FileName))
		dynamicText = atomic.NewString(downloadText)
		s.followVersion(sections, input, dynamicText)
	}

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	if picksVersion {
		options.ConfirmButton = constants.VirtualButtonX
	}
	if !settings.IsKidModeEnabled() {
		options.ActionButton = constants.VirtualButtonY
		options.AllowAction = true
	}

	result, err := gaba.DetailScreen(input.Game.Name, options, s.footer(picksVersion, downloadText, dynamicText))
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Detail screen error", "error", err)
		return output, err
	}

	switch result.Action {
	case gaba.DetailActionConfirmed:
		output.Action = GameDetailsActionDownload
		output.DownloadRequested = true
		output.SelectedFileID = selectedFileID(result.DropdownSelections)
	case gaba.DetailActionTriggered:
		output.Action = GameDetailsActionOptions
	}

	return output, nil
}

func (s *GameDetailsScreen) footer(picksVersion bool, downloadText string, dynamicText *atomic.String) []gaba.FooterHelpItem {
	items := []gaba.FooterHelpItem{FooterBack()}

	if !settings.IsKidModeEnabled() {
		items = append(items, gaba.FooterHelpItem{ButtonName: "Y", HelpText: localize("button_options", "Options")})
	}

	button := "A"
	if picksVersion {
		button = "X"
	}
	return append(items, gaba.FooterHelpItem{
		ButtonName:      button,
		HelpText:        downloadText,
		HelpTextDynamic: dynamicText,
	})
}

// downloadLabel says whether pressing the button fetches something new or
// replaces what is already on the card.
func downloadLabel(have bool) string {
	if have {
		return localize("button_redownload", "Redownload")
	}
	return localize("button_download", "Download")
}

// followVersion keeps the footer in step with the version picker.
func (s *GameDetailsScreen) followVersion(sections []gaba.Section, input GameDetailsInput, text *atomic.String) {
	for i := range sections {
		if sections[i].DropdownID != fileVersionDropdown {
			continue
		}
		sections[i].OnChange = func(option gaba.DropdownOption) {
			for _, file := range input.Game.Files {
				if strconv.Itoa(file.ID) == option.Value {
					text.Store(downloadLabel(catalog.IsFileDownloaded(*input.Config, input.Game, file.FileName)))
					return
				}
			}
		}
		return
	}
}

func selectedFileID(selections []gaba.DropdownSelection) int {
	for _, selection := range selections {
		if selection.ID == fileVersionDropdown {
			id, _ := strconv.Atoi(selection.Option.Value)
			return id
		}
	}
	return 0
}

func (s *GameDetailsScreen) sections(input GameDetailsInput, picksVersion bool) []gaba.Section {
	game := input.Game
	var sections []gaba.Section

	if cover := cache.ArtworkPath(game, input.Config.ArtKind, input.Host); cover != "" {
		sections = append(sections, gaba.NewImageSection("", cover, 640, 480, constants.TextAlignCenter))
	}

	if picksVersion {
		options := make([]gaba.DropdownOption, len(game.Files))
		for i, file := range game.Files {
			label := file.FileName
			if catalog.IsFileDownloaded(*input.Config, game, file.FileName) {
				label = constants.Download + " " + label
			}
			options[i] = gaba.DropdownOption{Label: label, Value: strconv.Itoa(file.ID)}
		}
		sections = append(sections, gaba.NewDropdownSection(
			localize("game_details_file_version", "File Version"), fileVersionDropdown, options, 0))
	}

	if game.Summary != "" {
		sections = append(sections, gaba.NewDescriptionSection("", game.Summary))
	}

	if facts := gameFacts(game); len(facts) > 0 {
		sections = append(sections, gaba.NewInfoSection("", facts))
	}

	// A game RomM knows nothing about would otherwise be a blank screen.
	if len(sections) == 0 {
		gaba.GetLogger().Warn("No details available for game", "game", game.Name)
		sections = append(sections, gaba.NewInfoSection("", []gaba.MetadataItem{
			{Label: localize("game_details_game", "Game"), Value: game.Name},
			{Label: localize("game_details_platform", "Platform"), Value: game.PlatformDisplayName},
		}))
	}

	return sections
}

// factRows are the metadata rows a game can show, in the order they appear.
// A row whose value comes back empty is left out, so a game RomM knows little
// about does not show a column of blanks.
var factRows = []struct {
	id, fallback string
	value        func(romm.Rom) string
}{
	{"game_details_release_date", "Release Date", func(g romm.Rom) string {
		if g.Metadatum.FirstReleaseDate <= 0 {
			return ""
		}
		// RomM reports this in milliseconds.
		return time.Unix(g.Metadatum.FirstReleaseDate/1000, 0).Format("January 2, 2006")
	}},
	{"game_details_average_rating", "Average Rating", func(g romm.Rom) string {
		if g.Metadatum.AverageRating <= 0 {
			return ""
		}
		return fmt.Sprintf("%.1f/100", g.Metadatum.AverageRating)
	}},
	{"game_details_genres", "Genres", func(g romm.Rom) string { return list(g.Metadatum.Genres) }},
	{"game_details_companies", "Companies", func(g romm.Rom) string { return list(g.Metadatum.Companies) }},
	{"game_details_game_modes", "Game Modes", func(g romm.Rom) string { return list(g.Metadatum.GameModes) }},
	{"game_details_regions", "Regions", func(g romm.Rom) string { return list(g.Regions) }},
	{"game_details_languages", "Languages", func(g romm.Rom) string { return list(g.Languages) }},
	{"game_details_file_size", "File Size", func(g romm.Rom) string {
		if g.FsSizeBytes <= 0 {
			return ""
		}
		return textmatch.FormatBytes(int64(g.FsSizeBytes))
	}},
	{"game_details_type", "Type", func(g romm.Rom) string {
		if !g.HasMultipleFiles {
			return ""
		}
		return localize("game_details_multi_file_rom", "Multi-file ROM")
	}},
}

func gameFacts(game romm.Rom) []gaba.MetadataItem {
	facts := make([]gaba.MetadataItem, 0, len(factRows))
	for _, row := range factRows {
		if value := row.value(game); value != "" {
			facts = append(facts, gaba.MetadataItem{Label: localize(row.id, row.fallback), Value: value})
		}
	}
	return facts
}

func list(values []string) string { return strings.Join(values, ", ") }

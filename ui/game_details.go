package ui

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	groutConstants "grout/constants"
	"grout/romm"
	"grout/utils"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type GameDetailsInput struct {
	Config   *utils.Config
	Host     romm.Host
	Platform romm.Platform
	Game     romm.Rom
}

type GameDetailsOutput struct {
	DownloadRequested bool
	Game              romm.Rom
	Platform          romm.Platform
}

type GameDetailsScreen struct{}

func NewGameDetailsScreen() *GameDetailsScreen {
	return &GameDetailsScreen{}
}

func (s *GameDetailsScreen) Draw(input GameDetailsInput) (ScreenResult[GameDetailsOutput], error) {
	logger := gaba.GetLogger()
	output := GameDetailsOutput{
		Game:     input.Game,
		Platform: input.Platform,
	}

	sections := s.buildSections(input)

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	if !utils.IsKidModeEnabled() {
		options.ActionButton = constants.VirtualButtonX
		options.EnableAction = true
	}

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)},
	}
	if !utils.IsKidModeEnabled() {
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_options", Other: "Options"}, nil)})
	}
	footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "A", HelpText: i18n.Localize(&goi18n.Message{ID: "button_download", Other: "Download"}, nil)})

	result, err := gaba.DetailScreen(input.Game.Name, options, footerItems)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		logger.Error("Detail screen error", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	if result.Action == gaba.DetailActionConfirmed {
		output.DownloadRequested = true
		return success(output), nil
	}

	if result.Action == gaba.DetailActionTriggered {
		return withCode(output, groutConstants.ExitCodeGameOptions), nil
	}

	return back(output), nil
}

func (s *GameDetailsScreen) buildSections(input GameDetailsInput) []gaba.Section {
	sections := make([]gaba.Section, 0)
	game := input.Game
	logger := gaba.GetLogger()

	coverImagePath := s.getCoverImagePath(input.Host, game)
	if coverImagePath != "" {
		sections = append(sections, gaba.NewImageSection("", coverImagePath, 640, 480, constants.TextAlignCenter))
	} else {
		logger.Debug("No cover image available", "game", game.Name)
	}

	if game.Summary != "" {
		sections = append(sections, gaba.NewDescriptionSection("", game.Summary))
	}

	metadata := make([]gaba.MetadataItem, 0)

	if game.Metadatum.FirstReleaseDate > 0 {
		releaseDate := time.Unix(game.Metadatum.FirstReleaseDate/1000, 0)
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_release_date", Other: "Release Date"}, nil),
			Value: releaseDate.Format("January 2, 2006"),
		})
	}

	if game.Metadatum.AverageRating > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_average_rating", Other: "Average Rating"}, nil),
			Value: fmt.Sprintf("%.1f/100", game.Metadatum.AverageRating),
		})
	}

	if len(game.Metadatum.Genres) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_genres", Other: "Genres"}, nil),
			Value: strings.Join(game.Metadatum.Genres, ", "),
		})
	}

	if len(game.Metadatum.Companies) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_companies", Other: "Companies"}, nil),
			Value: strings.Join(game.Metadatum.Companies, ", "),
		})
	}

	if len(game.Metadatum.GameModes) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_game_modes", Other: "Game Modes"}, nil),
			Value: strings.Join(game.Metadatum.GameModes, ", "),
		})
	}

	if len(game.Regions) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_regions", Other: "Regions"}, nil),
			Value: strings.Join(game.Regions, ", "),
		})
	}

	if len(game.Languages) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_languages", Other: "Languages"}, nil),
			Value: strings.Join(game.Languages, ", "),
		})
	}

	if game.FsSizeBytes > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_file_size", Other: "File Size"}, nil),
			Value: utils.FormatBytes(game.FsSizeBytes),
		})
	}

	if game.HasMultipleFiles {
		metadata = append(metadata, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "game_details_type", Other: "Type"}, nil),
			Value: i18n.Localize(&goi18n.Message{ID: "game_details_multi_file_rom", Other: "Multi-file ROM"}, nil),
		})
	}

	if len(metadata) > 0 {
		sections = append(sections, gaba.NewInfoSection("", metadata))
	}

	if len(sections) == 0 {
		logger.Warn("No sections available for game", "game", game.Name)
		sections = append(sections, gaba.NewInfoSection("", []gaba.MetadataItem{
			{Label: i18n.Localize(&goi18n.Message{ID: "game_details_game", Other: "Game"}, nil), Value: game.Name},
			{Label: i18n.Localize(&goi18n.Message{ID: "game_details_platform", Other: "Platform"}, nil), Value: game.PlatformDisplayName},
		}))
	}

	qrcode, err := utils.CreateTempQRCode(game.GetGamePage(input.Host), 256)
	if err == nil {
		sections = append(sections, gaba.NewImageSection(
			i18n.Localize(&goi18n.Message{ID: "game_details_qr_section", Other: "RomM Game Listing"}, nil),
			qrcode,
			int32(256),
			int32(256),
			constants.TextAlignCenter,
		))

	} else {
		logger.Error("Unable to generate QR code", "error", err)
	}

	return sections
}

// getCoverImagePath returns the path to the cover image, using cache if available
func (s *GameDetailsScreen) getCoverImagePath(host romm.Host, game romm.Rom) string {
	logger := gaba.GetLogger()

	// First, check if artwork is in the cache
	if utils.ArtworkExists(game.PlatformSlug, game.ID) {
		cachePath := utils.GetArtworkCachePath(game.PlatformSlug, game.ID)
		logger.Debug("Using cached artwork for game details", "game", game.Name)
		return cachePath
	}

	// Not in cache, fetch from server
	var coverPath string
	if game.PathCoverLarge != "" {
		coverPath = game.PathCoverLarge
	} else if game.PathCoverSmall != "" {
		coverPath = game.PathCoverSmall
	} else if game.URLCover != "" {
		coverPath = game.URLCover
	} else {
		return ""
	}

	coverURL := host.URL() + coverPath
	imageData := s.fetchImageFromURL(host, coverURL)

	// Cache the artwork for future use and return cache path
	if imageData != nil {
		if err := utils.EnsureArtworkCacheDir(game.PlatformSlug); err == nil {
			cachePath := utils.GetArtworkCachePath(game.PlatformSlug, game.ID)
			if err := os.WriteFile(cachePath, imageData, 0644); err == nil {
				utils.ProcessArtImage(cachePath)
				return cachePath
			}
		}
	}

	return ""
}

func (s *GameDetailsScreen) fetchImageFromURL(host romm.Host, imageURL string) []byte {
	logger := gaba.GetLogger()

	imageURL = strings.ReplaceAll(imageURL, " ", "%20")

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		logger.Warn("Failed to create image request", "url", imageURL, "error", err)
		return nil
	}

	req.SetBasicAuth(host.Username, host.Password)

	client := &http.Client{Timeout: groutConstants.DefaultHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		logger.Warn("Failed to fetch image", "url", imageURL, "error", err)
		return nil
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warn("Failed to close response body", "error", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Warn("Image fetch failed with bad status", "url", imageURL, "status", resp.Status)
		return nil
	}

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Warn("Failed to read image data", "error", err)
		return nil
	}

	logger.Debug("Successfully fetched image", "url", imageURL, "size", len(imageData))
	return imageData
}

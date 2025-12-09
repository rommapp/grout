package ui

import (
	"encoding/base64"
	"errors"
	"fmt"
	"grout/models"
	"grout/romm"
	"grout/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/UncleJunVIP/gabagool/v2/pkg/gabagool/constants"
)

type GameDetailsInput struct {
	Config   *models.Config
	Host     models.Host
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
	options.ShowThemeBackground = true
	options.ShowScrollbar = true

	result, err := gaba.DetailScreen(input.Game.Name, options, []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: "Back"},
		{ButtonName: "A", HelpText: "Download"},
	})

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return Back(output), nil
		}
		logger.Error("Detail screen error", "error", err)
		return WithCode(output, gaba.ExitCodeError), err
	}

	if result.Action == gaba.DetailActionConfirmed {
		output.DownloadRequested = true
		return Success(output), nil
	}

	return Back(output), nil
}

func (s *GameDetailsScreen) buildSections(input GameDetailsInput) []gaba.Section {
	sections := make([]gaba.Section, 0)
	game := input.Game
	logger := gaba.GetLogger()

	coverImageData := s.fetchCoverImage(input.Host, game)
	if coverImageData != nil {
		detailsDir := filepath.Join(utils.TempDir(), "details")
		tempImagePath := filepath.Join(detailsDir, fmt.Sprintf("grout_cover_%d.jpg", game.ID))

		if err := os.MkdirAll(detailsDir, 0755); err != nil {
			logger.Warn("Failed to create details directory", "error", err)
		} else {
			if err := os.Chmod(detailsDir, 0755); err != nil {
				logger.Warn("Failed to set directory permissions", "error", err)
			}

			err := os.WriteFile(tempImagePath, coverImageData, 0644)
			if err == nil {
				sections = append(sections, gaba.NewImageSection("", tempImagePath, 640, 480, constants.TextAlignCenter))
			} else {
				logger.Warn("Failed to write cover image", "error", err)
			}
		}
	}

	if game.Summary != "" {
		sections = append(sections, gaba.NewDescriptionSection("", game.Summary))
	}

	metadata := make([]gaba.MetadataItem, 0)

	if game.Metadatum.FirstReleaseDate > 0 {
		releaseDate := time.Unix(game.Metadatum.FirstReleaseDate/1000, 0)
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Release Date",
			Value: releaseDate.Format("January 2, 2006"),
		})
	}

	if game.Metadatum.AverageRating > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Average Rating",
			Value: fmt.Sprintf("%.1f/100", game.Metadatum.AverageRating),
		})
	}

	if len(game.Metadatum.Genres) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Genres",
			Value: strings.Join(game.Metadatum.Genres, ", "),
		})
	}

	if len(game.Metadatum.Companies) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Companies",
			Value: strings.Join(game.Metadatum.Companies, ", "),
		})
	}

	if len(game.Metadatum.GameModes) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Game Modes",
			Value: strings.Join(game.Metadatum.GameModes, ", "),
		})
	}

	if len(game.Regions) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Regions",
			Value: strings.Join(game.Regions, ", "),
		})
	}

	if len(game.Languages) > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Languages",
			Value: strings.Join(game.Languages, ", "),
		})
	}

	if game.FsSizeBytes > 0 {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "File Size",
			Value: formatBytes(game.FsSizeBytes),
		})
	}

	if game.Multi {
		metadata = append(metadata, gaba.MetadataItem{
			Label: "Type",
			Value: "Multi-file ROM",
		})
	}

	if len(metadata) > 0 {
		sections = append(sections, gaba.NewInfoSection("", metadata))
	}

	if len(sections) == 0 {
		logger.Warn("No sections available for game", "game", game.Name)
		sections = append(sections, gaba.NewInfoSection("", []gaba.MetadataItem{
			{Label: "Game", Value: game.Name},
			{Label: "Platform", Value: game.PlatformDisplayName},
		}))
	}

	qrcode, err := utils.CreateTempQRCode(game.GetGamePage(input.Host), 256)
	if err == nil {
		sections = append(sections, gaba.NewImageSection(
			"RomM Game Listing",
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

func (s *GameDetailsScreen) fetchCoverImage(host models.Host, game romm.Rom) []byte {
	var coverPath string
	if game.PathCoverLarge != "" {
		coverPath = game.PathCoverLarge
	} else if game.URLCover != "" {
		coverPath = game.URLCover
	} else {
		return nil
	}

	coverURL := host.URL() + coverPath
	return s.fetchImageFromURL(host, coverURL)
}

func (s *GameDetailsScreen) fetchImageFromURL(host models.Host, imageURL string) []byte {
	logger := gaba.GetLogger()

	imageURL = strings.ReplaceAll(imageURL, " ", "%20")

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		logger.Warn("Failed to create image request", "url", imageURL, "error", err)
		return nil
	}

	auth := host.Username + ":" + host.Password
	authHeader := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 10 * time.Second}
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

func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

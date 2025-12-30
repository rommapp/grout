package ui

import (
	"errors"
	"grout/romm"
	"grout/utils"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type GameOptionsInput struct {
	Config *utils.Config
	Game   romm.Rom
}

type GameOptionsOutput struct {
	Config *utils.Config
}

type GameOptionsScreen struct{}

func NewGameOptionsScreen() *GameOptionsScreen {
	return &GameOptionsScreen{}
}

func (s *GameOptionsScreen) Draw(input GameOptionsInput) (ScreenResult[GameOptionsOutput], error) {
	config := input.Config
	output := GameOptionsOutput{Config: config}

	items := s.buildMenuItems(config, input.Game)

	if len(items) == 0 {
		gaba.GetLogger().Warn("No options available for game")
		return back(output), nil
	}

	title := i18n.Localize(&goi18n.Message{ID: "game_options_title", Other: "Game Options"}, nil)

	result, err := gaba.OptionsList(
		title,
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
			InitialSelectedIndex: 0,
			StatusBar:            utils.StatusBar(),
			SmallTitle:           true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		gaba.GetLogger().Error("Game options screen error", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	s.applySettings(config, input.Game, result.Items)

	err = utils.SaveConfig(config)
	if err != nil {
		gaba.GetLogger().Error("Error saving game options", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	return success(output), nil
}

func (s *GameOptionsScreen) buildMenuItems(config *utils.Config, game romm.Rom) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0)

	// Save Directory option
	saveDirectories := utils.EmulatorFoldersForSlug(game.PlatformSlug)
	if len(saveDirectories) > 0 {
		options := make([]gaba.Option, 0, len(saveDirectories)+1)

		// Add "Default" option first
		options = append(options, gaba.Option{
			DisplayName: i18n.Localize(&goi18n.Message{ID: "common_default", Other: "Default"}, nil),
			Value:       "",
		})

		// Add each emulator directory as an option
		for _, dir := range saveDirectories {
			options = append(options, gaba.Option{
				DisplayName: dir,
				Value:       dir,
			})
		}

		// Determine currently selected option
		selectedIndex := 0
		if config.GameSaveOverrides != nil {
			if currentOverride, ok := config.GameSaveOverrides[game.ID]; ok && currentOverride != "" {
				for i, opt := range options {
					if val, ok := opt.Value.(string); ok && val == currentOverride {
						selectedIndex = i
						break
					}
				}
			}
		}

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: "game_options_save_directory", Other: "Save Directory"}, nil)},
			Options:        options,
			SelectedOption: selectedIndex,
		})
	}

	return items
}

func (s *GameOptionsScreen) applySettings(config *utils.Config, game romm.Rom, items []gaba.ItemWithOptions) {
	logger := gaba.GetLogger()

	for _, item := range items {
		text := item.Item.Text

		if text == i18n.Localize(&goi18n.Message{ID: "game_options_save_directory", Other: "Save Directory"}, nil) {
			newDir, ok := item.Options[item.SelectedOption].Value.(string)
			if !ok {
				continue
			}

			// Get the current override (if any)
			var oldDir string
			if config.GameSaveOverrides != nil {
				oldDir = config.GameSaveOverrides[game.ID]
			}

			// Resolve actual directories (empty string means default/first in list)
			saveDirectories := utils.EmulatorFoldersForSlug(game.PlatformSlug)
			if len(saveDirectories) == 0 {
				continue
			}

			resolvedOldDir := oldDir
			if resolvedOldDir == "" {
				resolvedOldDir = saveDirectories[0]
			}

			resolvedNewDir := newDir
			if resolvedNewDir == "" {
				resolvedNewDir = saveDirectories[0]
			}

			// Move save file if directory changed
			if resolvedOldDir != resolvedNewDir {
				s.moveSaveFile(game, resolvedOldDir, resolvedNewDir)
			}

			// Update config
			if config.GameSaveOverrides == nil {
				config.GameSaveOverrides = make(map[int]string)
			}

			if newDir == "" {
				delete(config.GameSaveOverrides, game.ID)
			} else {
				config.GameSaveOverrides[game.ID] = newDir
			}

			logger.Debug("Save directory changed", "game", game.Name, "oldDir", resolvedOldDir, "newDir", resolvedNewDir)
		}
	}
}

func (s *GameOptionsScreen) moveSaveFile(game romm.Rom, oldDir, newDir string) {
	logger := gaba.GetLogger()
	basePath := utils.BaseSavePath()

	// Get the game's base name (ROM filename without extension)
	gameBase := strings.TrimSuffix(game.FsNameNoExt, filepath.Ext(game.FsNameNoExt))
	if gameBase == "" {
		gameBase = game.Name
	}

	oldDirPath := filepath.Join(basePath, oldDir)
	newDirPath := filepath.Join(basePath, newDir)

	// Look for save files in the old directory that match this game
	entries, err := os.ReadDir(oldDirPath)
	if err != nil {
		logger.Debug("Could not read old save directory", "path", oldDirPath, "error", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		fileBase := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		if fileBase == gameBase {
			oldPath := filepath.Join(oldDirPath, fileName)
			newPath := filepath.Join(newDirPath, fileName)

			// Ensure new directory exists
			if err := os.MkdirAll(newDirPath, 0755); err != nil {
				logger.Error("Failed to create new save directory", "path", newDirPath, "error", err)
				return
			}

			// Move the file
			if err := os.Rename(oldPath, newPath); err != nil {
				logger.Error("Failed to move save file", "from", oldPath, "to", newPath, "error", err)
				return
			}

			logger.Info("Moved save file", "from", oldPath, "to", newPath)
		}
	}
}

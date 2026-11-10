package ui

import (
	"errors"
	"grout/catalog"
	"grout/cfw"
	"grout/romm"
	"grout/saves"
	"grout/settings"
	"os"
	"path/filepath"
	"strings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type GameOptionsInput struct {
	Config *settings.Config
	Host   settings.Host
	Game   romm.Rom
	// SlotNames, once SlotsLoaded, is the server's slots from an earlier draw
	// of this panel, so coming back from the QR code does not fetch them again.
	SlotNames   []string
	SlotsLoaded bool
}

type GameOptionsOutput struct {
	Action      GameOptionsAction
	Config      *settings.Config
	Host        settings.Host
	Game        romm.Rom
	NewSlotName string // Set when a new slot is created (for targeted upload)
	SlotNames   []string
	SlotsLoaded bool
}

type GameOptionsScreen struct{}

func NewGameOptionsScreen() *GameOptionsScreen {
	return &GameOptionsScreen{}
}

func (s *GameOptionsScreen) Draw(input GameOptionsInput) (GameOptionsOutput, error) {
	config := input.Config
	output := GameOptionsOutput{Action: GameOptionsActionBack, Config: config, Host: input.Host, Game: input.Game}

	// Fetch save summary to determine available slots
	slotNames := input.SlotNames
	if input.Host.DeviceID != "" && !input.SlotsLoaded {
		client := romm.NewClientFromHost(input.Host, config.ApiTimeout.Duration())
		gaba.ProcessMessage(
			localize("synced_games_loading_detail", "Loading save details..."),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (any, error) {
				summary, err := client.GetSaveSummary(input.Game.ID)
				if err == nil {
					slotNames = saves.SlotNames(summary)
				}
				return nil, nil
			},
		)
	}

	output.SlotNames, output.SlotsLoaded = slotNames, true

	oldSlotPref := config.GetSlotPreference(input.Game.ID)

	// A game's core can only be changed once the game is on the device, and
	// only where the firmware lets a single game change core.
	romPath := catalog.LocalRomPath(*config, input.Game)
	cores, hasCores := cfw.ActiveFirmware().GameCores(*config, input.Game.PlatformFSSlug, romPath)

	items := s.buildMenuItems(config, input.Game, input.Host.DeviceID != "", slotNames)
	if hasCores {
		items = append(items, gameCoreItem(cores))
	}

	// Saves for this game in more than one emulator's folder: the sync leaves
	// the game alone until the player says which is the right one.
	var inDoubt []saves.DuplicateSaves
	resolveText := localize("game_options_resolve_saves", "Resolve Saves")
	if duplicates := saves.FindDuplicateSaves(saves.LocalSavesFor(config, input.Game.ID)); len(duplicates) > 0 {
		inDoubt = duplicates
		items = append([]gaba.ItemWithOptions{{
			Item:           gaba.MenuItem{Text: resolveText},
			Options:        []gaba.Option{{DisplayName: "", Value: "resolve_saves", Type: gaba.OptionTypeClickable}},
			SelectedOption: 0,
		}}, items...)
	}

	showQRText := localize("game_options_show_qr", "Show QR Code")
	items = append(items, gaba.ItemWithOptions{
		Item:           gaba.MenuItem{Text: showQRText},
		Options:        []gaba.Option{{DisplayName: "", Value: "show_qr", Type: gaba.OptionTypeClickable}},
		SelectedOption: 0,
	})

	title := localize("game_options_title", "Game Options")

	result, err := gaba.OptionsList(
		title,
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
			InitialSelectedIndex: 0,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		gaba.GetLogger().Error("Game options screen error", "error", err)
		return output, err
	}

	if result.Action == gaba.ListActionSelected {
		if result.Selected >= 0 && result.Selected < len(result.Items) {
			selectedItem := result.Items[result.Selected]
			if selectedItem.Item.Text == showQRText {
				output.Action = GameOptionsActionShowQR
				return output, nil
			}
			if selectedItem.Item.Text == resolveText && len(inDoubt) > 0 {
				s.resolveSaves(input.Host, inDoubt[0], romPath)
				output.Action = GameOptionsActionSaved
				return output, nil
			}
		}
	}

	s.applySettings(config, input.Game, result.Items)
	coreChanged := hasCores && s.applyGameCore(config, input.Game, romPath, cores, result.Items)

	if err = settings.SaveSlotPreferences(config); err != nil {
		gaba.GetLogger().Error("Error saving slot preferences", "error", err)
		return output, err
	}

	newSlotPref := config.GetSlotPreference(input.Game.ID)
	if coreChanged && input.Host.DeviceID != "" && newSlotPref == oldSlotPref {
		// The game now reads another core's save folder, so a sync fetches the
		// save it will find there.
		output.Action = GameOptionsActionSyncNow
	} else if newSlotPref != oldSlotPref {
		output.Action = GameOptionsActionSyncNow
		// Check if this is a brand-new slot (not on server yet) for targeted upload
		isNewSlot := true
		for _, name := range slotNames {
			if name == newSlotPref {
				isNewSlot = false
				break
			}
		}
		if isNewSlot {
			output.NewSlotName = newSlotPref
		}
	} else {
		output.Action = GameOptionsActionSaved
	}
	return output, nil
}

func (s *GameOptionsScreen) buildMenuItems(config *settings.Config, game romm.Rom, deviceRegistered bool, slotNames []string) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0)

	if deviceRegistered {
		saveSlotText := localize("game_options_save_slot", "Save Slot")
		slotOpts := BuildSlotOptions(config, game.ID, slotNames)

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: saveSlotText},
			Options:        slotOpts.Options,
			SelectedOption: slotOpts.SelectedIdx,
		})
	}

	return items
}

func (s *GameOptionsScreen) applySettings(config *settings.Config, game romm.Rom, items []gaba.ItemWithOptions) {
	saveSlotText := localize("game_options_save_slot", "Save Slot")

	for _, item := range items {
		if item.Item.Text == saveSlotText {
			if item.SelectedOption >= 0 && item.SelectedOption < len(item.Options) {
				selectedOpt := item.Options[item.SelectedOption]
				// Empty string values come from the "New Slot..." keyboard option
				// when the user dismisses the keyboard without typing. Intentionally
				// treated as a no-op so the preference remains unchanged.
				if selectedSlot, ok := selectedOpt.Value.(string); ok && selectedSlot != "" {
					config.SetSlotPreference(game.ID, selectedSlot)
				}
			}
		}
	}
}

// gameCoreItem is the row that picks which core the game runs on. The system's
// default is marked, since choosing it is how a game goes back to the usual.
func gameCoreItem(cores cfw.GameCores) gaba.ItemWithOptions {
	options := make([]gaba.Option, 0, len(cores.Choices))
	selected := 0
	for i, choice := range cores.Choices {
		name := choice.Name
		if choice.ID == cores.Default {
			name = localizeWith("game_core_default", "{{.Name}} (Default)", map[string]any{"Name": choice.Name})
		}
		options = append(options, gaba.Option{DisplayName: name, Value: choice.ID})
		if choice.ID == cores.Current {
			selected = i
		}
	}
	return gaba.ItemWithOptions{
		Item:           gaba.MenuItem{Text: localize("game_options_game_core", "Game Core")},
		Options:        options,
		SelectedOption: selected,
	}
}

// applyGameCore gives the game the core that was picked, and reports whether
// that changed anything.
//
// Each core keeps its saves in a folder of its own, so a game moved to another
// core would start without the save it had. If the game has a save in another
// folder and none in the new one, Grout offers to move it across.
func (s *GameOptionsScreen) applyGameCore(config *settings.Config, game romm.Rom, romPath string, cores cfw.GameCores, items []gaba.ItemWithOptions) bool {
	logger := gaba.GetLogger()
	coreText := localize("game_options_game_core", "Game Core")

	for _, item := range items {
		if item.Item.Text != coreText || item.SelectedOption < 0 || item.SelectedOption >= len(item.Options) {
			continue
		}
		chosen, _ := item.Options[item.SelectedOption].Value.(string)
		if chosen == "" || chosen == cores.Current {
			return false
		}

		if err := cfw.ActiveFirmware().SetGameCore(romPath, chosen); err != nil {
			logger.Error("Failed to change the game's core", "rom", romPath, "core", chosen, "error", err)
			gaba.ConfirmationMessage(
				localize("game_core_change_failed", "Could not change the game's core.\nSee the Grout logs."),
				ContinueFooter(),
				gaba.MessageOptions{},
			)
			return false
		}
		logger.Info("Changed the game's core", "rom", romPath, "from", cores.Current, "to", chosen)

		for _, choice := range cores.Choices {
			if choice.ID == chosen {
				s.offerToMoveSave(config, game, choice.Name)
			}
		}
		return true
	}
	return false
}

// offerToMoveSave asks whether to move the game's save to the folder of the
// core it now runs on.
func (s *GameOptionsScreen) offerToMoveSave(config *settings.Config, game romm.Rom, newFolder string) {
	logger := gaba.GetLogger()

	toMove := saves.SavesToCarry(saves.LocalSavesFor(config, game.ID), newFolder)
	if len(toMove) == 0 {
		return
	}
	from := filepath.Base(filepath.Dir(toMove[0].FilePath))

	_, err := gaba.ConfirmationMessage(
		localizeWith("game_core_move_save", "Move this game's save from {{.From}} to {{.To}}?\nThe original is kept in a backup folder.",
			map[string]any{"From": from, "To": newFolder}),
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: localize("button_cancel", "Cancel")},
			{ButtonName: "X", HelpText: localize("button_confirm", "Confirm")},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	if err != nil {
		if !errors.Is(err, gaba.ErrCancelled) {
			logger.Error("Move-save prompt failed", "error", err)
		}
		return
	}

	destDir := filepath.Join(cfw.BaseSavePath(), newFolder)
	for _, save := range toMove {
		if err := saves.CarrySave(save, destDir); err != nil {
			logger.Error("Failed to move the save", "save", save.FilePath, "to", destDir, "error", err)
			gaba.ConfirmationMessage(
				localize("game_core_move_failed", "Could not move the save.\nSee the Grout logs."),
				ContinueFooter(),
				gaba.MessageOptions{},
			)
			return
		}
		logger.Info("Moved the save to the new core's folder", "save", save.FilePath, "to", destDir)
	}
}

// resolveSaves asks which copy of a game's save is the right one and keeps it.
//
// The sync cannot tell, so it is the player who says. Each copy is described by
// the folder it is in, when it was last written, and whether it has changed
// since it was last synced with RomM, which is usually the clue: the copy that
// matches what RomM sent is the one that has not been played since.
func (s *GameOptionsScreen) resolveSaves(host settings.Host, duplicates saves.DuplicateSaves, romPath string) {
	logger := gaba.GetLogger()

	options := make([]gaba.Option, 0, len(duplicates.Copies))
	for i, copy := range duplicates.Copies {
		label := localizeWith("resolve_saves_copy", "{{.Folder}}: {{.Modified}}, {{.Size}} KB",
			map[string]any{"Folder": saves.FolderName(copy), "Modified": modifiedAt(copy), "Size": sizeKB(copy)})
		if changed, known := saves.ChangedSinceSync(host.DeviceID, copy); known {
			if changed {
				label += " " + localize("resolve_saves_changed", "(played since last sync)")
			} else {
				label += " " + localize("resolve_saves_unchanged", "(as last synced)")
			}
		}
		options = append(options, gaba.Option{DisplayName: label, Value: i})
	}

	result, err := gaba.OptionsList(
		localize("resolve_saves_title", "Which Save Is Current?"),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		[]gaba.ItemWithOptions{{
			Item:    gaba.MenuItem{Text: localize("resolve_saves_keep", "Use")},
			Options: options,
		}},
	)
	if err != nil {
		if !errors.Is(err, gaba.ErrCancelled) {
			logger.Error("Resolve saves screen error", "error", err)
		}
		return
	}
	if len(result.Items) == 0 || result.Items[0].SelectedOption < 0 || result.Items[0].SelectedOption >= len(options) {
		return
	}
	keep := result.Items[0].SelectedOption
	chosen := duplicates.Copies[keep]

	// It goes where the game's core reads it, or stays in its own folder when
	// that cannot be told.
	folder := cfw.ActiveFirmware().SaveFolderForRom(romPath)
	destDir := filepath.Dir(chosen.FilePath)
	if folder != "" {
		for _, copy := range duplicates.Copies {
			if known := saves.FolderName(copy); strings.EqualFold(known, folder) {
				destDir = filepath.Dir(copy.FilePath)
			}
		}
	}

	_, err = gaba.ConfirmationMessage(
		localizeWith("resolve_saves_confirm", "Use the save from {{.From}} as this game's save?\nThe other copies are kept in backup folders.",
			map[string]any{"From": saves.FolderName(chosen)}),
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: localize("button_cancel", "Cancel")},
			{ButtonName: "X", HelpText: localize("button_confirm", "Confirm")},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	if err != nil {
		return
	}

	if err := saves.KeepSave(duplicates, keep, destDir); err != nil {
		logger.Error("Failed to resolve the game's saves", "rom", duplicates.RomName, "error", err)
		gaba.ConfirmationMessage(
			localize("resolve_saves_failed", "Could not resolve the saves.\nSee the Grout logs."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}
	logger.Info("Resolved the game's saves", "rom", duplicates.RomName, "kept", saves.FolderName(chosen), "in", destDir)
	gaba.ConfirmationMessage(
		localize("resolve_saves_done", "Done. Sync to bring it in line with RomM."),
		ContinueFooter(),
		gaba.MessageOptions{},
	)
}

func modifiedAt(save saves.LocalSave) string {
	if info, err := os.Stat(save.FilePath); err == nil {
		return info.ModTime().Format("2006-01-02 15:04")
	}
	return "?"
}

func sizeKB(save saves.LocalSave) int64 {
	if info, err := os.Stat(save.FilePath); err == nil {
		return (info.Size() + 1023) / 1024
	}
	return 0
}

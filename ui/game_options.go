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
	"time"

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
	if duplicates := saves.FindDuplicateSaves(saves.LocalSavesFor(config, input.Game.ID)); len(duplicates) > 0 && cfw.ActiveFirmware().KeepsSavesPerCore() {
		inDoubt = duplicates
		items = append([]gaba.ItemWithOptions{{
			Item:           gaba.MenuItem{Text: resolveText},
			Options:        []gaba.Option{{DisplayName: "", Value: "resolve_saves", Type: gaba.OptionTypeClickable}},
			SelectedOption: 0,
		}}, items...)
	}

	// Taking a side by hand, whichever save is newer. Both are the player's call
	// and both ask first.
	useRommText := localize("game_options_use_romm", "Use RomM's Save")
	uploadText := localize("game_options_upload_local", "Upload Local Save")
	if input.Host.DeviceID != "" {
		for _, text := range []string{useRommText, uploadText} {
			items = append(items, gaba.ItemWithOptions{
				Item:           gaba.MenuItem{Text: text},
				Options:        []gaba.Option{{DisplayName: "", Value: text, Type: gaba.OptionTypeClickable}},
				SelectedOption: 0,
			})
		}
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
			if input.Host.DeviceID != "" && (selectedItem.Item.Text == useRommText || selectedItem.Item.Text == uploadText) {
				if selectedItem.Item.Text == useRommText {
					s.useRommsSave(config, input.Host, input.Game, romPath)
				} else {
					s.uploadLocalSave(config, input.Host, input.Game)
				}
				output.Action = GameOptionsActionSaved
				return output, nil
			}
			if selectedItem.Item.Text == resolveText && len(inDoubt) > 0 {
				s.resolveSaves(config, input.Host, inDoubt[0], romPath)
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
		save := save
		err := whileWorking(localize("game_core_moving_save", "Moving the save..."), func() error {
			return saves.CarrySave(save, destDir)
		})
		if err != nil {
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

// resolveSaves asks which save is the right one for a game that has copies in
// more than one folder, and keeps it. RomM's own save is one of the choices
// when it can be reached, because it is often the one that is wanted.
//
// Each copy is described by the folder it is in, when it was last written, and
// whether it has changed since it was last synced with RomM, which is usually
// the clue: the copy that matches what RomM sent is the one that has not been
// played since.
func (s *GameOptionsScreen) resolveSaves(config *settings.Config, host settings.Host, duplicates saves.DuplicateSaves, romPath string) {
	logger := gaba.GetLogger()

	var client *romm.Client
	var remote *romm.Save
	if host.DeviceID != "" {
		client = romm.NewClientFromHost(host, config.ApiTimeout.Duration())
		gaba.ProcessMessage(
			localize("resolve_saves_checking", "Checking RomM..."),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (any, error) {
				var err error
				if remote, err = saves.RemoteSaveFor(client, config, host.DeviceID, duplicates.RomID); err != nil {
					logger.Warn("Could not look up RomM's save for the game", "rom", duplicates.RomName, "error", err)
				}
				return nil, nil
			},
		)
	}

	options := make([]gaba.Option, 0, len(duplicates.Copies)+1)
	newestLocal := time.Time{}
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
		if info, err := os.Stat(copy.FilePath); err == nil && info.ModTime().After(newestLocal) {
			newestLocal = info.ModTime()
		}
	}

	selected := 0
	if remote != nil {
		options = append(options, gaba.Option{
			DisplayName: localizeWith("resolve_saves_romm", "RomM: {{.Modified}}, {{.Size}} KB",
				map[string]any{"Modified": remote.UpdatedAt.Local().Format("2006-01-02 15:04"), "Size": (remote.FileSizeBytes + 1023) / 1024}),
			Value: len(duplicates.Copies),
		})
		// Start on RomM's save when it is newer than anything on the device.
		if remote.UpdatedAt.After(newestLocal) {
			selected = len(duplicates.Copies)
		}
	}

	result, err := gaba.OptionsList(
		localize("resolve_saves_title", "Which Save Is Current?"),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		[]gaba.ItemWithOptions{{
			Item:           gaba.MenuItem{Text: localize("resolve_saves_keep", "Use")},
			Options:        options,
			SelectedOption: selected,
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
	choice := result.Items[0].SelectedOption

	// It goes where the game's core reads it, even if no copy is there yet (a game
	// can have copies under two old cores and none under the one it runs on), or
	// stays in its own folder when that cannot be told.
	destDir := filepath.Dir(duplicates.Copies[0].FilePath)
	if choice < len(duplicates.Copies) {
		destDir = filepath.Dir(duplicates.Copies[choice].FilePath)
	}
	if dir := saves.SaveDirectoryForGame(duplicates.Copies[0].FSSlug, romPath, config); dir != "" {
		destDir = dir
	}

	var message string
	if choice == len(duplicates.Copies) && remote != nil {
		message = localize("resolve_saves_confirm_romm", "Use RomM's save as this game's save?\nThe copies on this device are kept in backup folders.")
	} else {
		chosen := duplicates.Copies[choice]
		message = localizeWith("resolve_saves_confirm", "Use the save from {{.From}} as this game's save?\nThe other copies are kept in backup folders.",
			map[string]any{"From": saves.FolderName(chosen)})
		// Syncing next would send this copy, and RomM's newer save would lose.
		if info, err := os.Stat(chosen.FilePath); err == nil && remote != nil && remote.UpdatedAt.After(info.ModTime()) {
			message += "\n" + localizeWith("resolve_saves_romm_newer", "RomM has a newer save, from {{.When}}.\nSyncing may replace it with this one.",
				map[string]any{"When": remote.UpdatedAt.Local().Format("2006-01-02 15:04")})
		}
	}

	_, err = gaba.ConfirmationMessage(
		message,
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: localize("button_cancel", "Cancel")},
			{ButtonName: "X", HelpText: localize("button_confirm", "Confirm")},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	if err != nil {
		return
	}

	if choice == len(duplicates.Copies) && remote != nil {
		err = whileWorking(localize("force_downloading", "Getting RomM's save..."), func() error {
			return saves.UseRemoteSave(client, config, host.DeviceID, duplicates, remote, destDir)
		})
	} else {
		err = whileWorking(localize("resolve_saves_working", "Putting the save in place..."), func() error {
			return saves.KeepSave(duplicates, choice, destDir)
		})
	}
	if err != nil {
		logger.Error("Failed to resolve the game's saves", "rom", duplicates.RomName, "error", err)
		gaba.ConfirmationMessage(
			localize("resolve_saves_failed", "Could not resolve the saves.\nSee the Grout logs."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}
	logger.Info("Resolved the game's saves", "rom", duplicates.RomName, "choice", choice, "in", destDir)

	// The save the game reads has changed if it came from RomM or from another
	// folder, and the resume state still holds the progress of the one it replaced.
	// A copy that was already the one in use leaves the state consistent.
	chosenFromRomM := choice == len(duplicates.Copies)
	var chosenCopy *saves.LocalSave
	if !chosenFromRomM {
		chosenCopy = &duplicates.Copies[choice]
	}
	replacedTheSaveInUse := saves.ChangesSaveInUse(chosenCopy, destDir)
	if chosenFromRomM && remote != nil && saves.FollowSlot(config, duplicates.RomID, remote) {
		if err := settings.SaveSlotPreferences(config); err != nil {
			logger.Error("Error saving slot preferences", "error", err)
		}
	}

	done := localize("resolve_saves_done", "Done.")
	if replacedTheSaveInUse && romPath != "" {
		if moved, err := cfw.ActiveFirmware().SetAsideResumeState(romPath); err != nil {
			logger.Error("Could not set the resume state aside", "rom", romPath, "error", err)
		} else if moved > 0 {
			done = localize("resolve_saves_done_state", "Done. The game will start from this\nsave, not from its old resume state.")
		}
	}
	gaba.ConfirmationMessage(done, ContinueFooter(), gaba.MessageOptions{})
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

// confirmWarning asks the player to be sure, on a button they do not press by
// habit: B backs out and the confirm button is X, not the A that continues
// everywhere else.
func confirmWarning(message string) bool {
	_, err := gaba.ConfirmationMessage(
		message,
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: localize("button_cancel", "Cancel")},
			{ButtonName: "X", HelpText: localize("button_confirm_anyway", "I'm Sure")},
		},
		gaba.MessageOptions{ConfirmButton: buttons.VirtualButtonX},
	)
	return err == nil
}

// whileWorking keeps a message on screen for as long as the work takes. A
// confirmed choice otherwise shows nothing until the work is over, which on a
// handheld over Wi-Fi can be many seconds, and a player cannot tell a press that
// was received from one that was not.
func whileWorking(message string, work func() error) error {
	var err error
	gaba.ProcessMessage(
		message,
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			err = work()
			return nil, nil
		},
	)
	return err
}

func tell(message string) {
	gaba.ConfirmationMessage(message, ContinueFooter(), gaba.MessageOptions{})
}

// useRommsSave replaces the game's save on the device with RomM's newest, in
// whatever slot it is, whether or not RomM's is the newer of the two.
func (s *GameOptionsScreen) useRommsSave(config *settings.Config, host settings.Host, game romm.Rom, romPath string) {
	logger := gaba.GetLogger()
	client := romm.NewClientFromHost(host, config.ApiTimeout.Duration())

	var remote *romm.Save
	var lookupErr error
	gaba.ProcessMessage(
		localize("resolve_saves_checking", "Checking RomM..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			remote, lookupErr = saves.NewestRemoteSave(client, host.DeviceID, game.ID)
			return nil, nil
		},
	)
	if lookupErr != nil {
		logger.Error("Could not look up RomM's save", "game", game.Name, "error", lookupErr)
		tell(localize("force_romm_unreachable", "Could not reach RomM.\nSee the Grout logs."))
		return
	}
	if remote == nil {
		tell(localize("force_romm_has_none", "RomM has no save for\nthis game."))
		return
	}

	local := saves.LocalSavesFor(config, game.ID)
	dup := saves.DuplicateSaves{RomID: game.ID, RomName: game.Name, Copies: local, FSSlug: game.PlatformFSSlug}
	if romPath != "" {
		dup.RomFileName = filepath.Base(romPath)
	}

	// It goes where the game's core reads it; a game with a save already keeps
	// using the folder that save is in.
	destDir := saves.SaveDirectoryForGame(game.PlatformFSSlug, romPath, config)
	if destDir == "" && len(local) > 0 {
		destDir = filepath.Dir(local[0].FilePath)
	}
	if destDir == "" {
		destDir = saves.ResolveSaveDirectory(game.PlatformFSSlug, config)
	}
	if destDir == "" {
		tell(localize("force_no_folder", "Could not tell where this\ngame's save belongs."))
		return
	}

	message := localizeWith("force_download_warning",
		"Replace the save on this device with RomM's?\nRomM's: {{.Romm}}{{.Slot}}\n{{.Device}}\nThis ignores which is newer.\nThe current save is backed up.",
		map[string]any{
			"Romm":   remote.UpdatedAt.Local().Format("2006-01-02 15:04"),
			"Slot":   forceSlotNote(remote),
			"Device": deviceSaveLine(local),
		})
	if !confirmWarning(message) {
		return
	}

	err := whileWorking(localize("force_downloading", "Getting RomM's save..."), func() error {
		return saves.UseRemoteSave(client, config, host.DeviceID, dup, remote, destDir)
	})
	if err != nil {
		logger.Error("Failed to use RomM's save", "game", game.Name, "error", err)
		tell(localize("force_failed", "That did not work.\nSee the Grout logs."))
		return
	}
	logger.Info("Took RomM's save by choice", "game", game.Name, "saveID", remote.ID, "slot", saves.SlotOf(remote), "in", destDir)

	if saves.FollowSlot(config, game.ID, remote) {
		if err := settings.SaveSlotPreferences(config); err != nil {
			logger.Error("Error saving slot preferences", "error", err)
		}
	}
	done := localize("force_download_done", "Took RomM's save.")
	if romPath != "" {
		if moved, err := cfw.ActiveFirmware().SetAsideResumeState(romPath); err != nil {
			logger.Error("Could not set the resume state aside", "rom", romPath, "error", err)
		} else if moved > 0 {
			done += "\n" + localize("force_state_set_aside", "The old resume state was set aside.")
		}
	}
	tell(done)
}

// uploadLocalSave sends the device's save to RomM as the game's current save,
// whether or not RomM's is the newer of the two.
func (s *GameOptionsScreen) uploadLocalSave(config *settings.Config, host settings.Host, game romm.Rom) {
	logger := gaba.GetLogger()

	local := saves.LocalSavesFor(config, game.ID)
	if len(local) == 0 {
		tell(localize("force_no_local_save", "There is no save for this game\non this device."))
		return
	}
	if len(saves.FindDuplicateSaves(local)) > 0 && cfw.ActiveFirmware().KeepsSavesPerCore() {
		tell(localize("force_saves_in_two_places", "This game has saves in more\nthan one folder.\nPick one with Resolve Saves first."))
		return
	}

	client := romm.NewClientFromHost(host, config.ApiTimeout.Duration())
	var remote *romm.Save
	var lookupErr error
	gaba.ProcessMessage(
		localize("resolve_saves_checking", "Checking RomM..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			remote, lookupErr = saves.NewestRemoteSave(client, host.DeviceID, game.ID)
			return nil, nil
		},
	)
	if lookupErr != nil {
		logger.Error("Could not look up RomM's save", "game", game.Name, "error", lookupErr)
		tell(localize("force_romm_unreachable", "Could not reach RomM.\nSee the Grout logs."))
		return
	}

	rommLine := localize("force_romm_none_yet", "RomM has no save yet.")
	if remote != nil {
		rommLine = localizeWith("force_romm_has", "RomM's: {{.When}}{{.Slot}}",
			map[string]any{"When": remote.UpdatedAt.Local().Format("2006-01-02 15:04"), "Slot": forceSlotNote(remote)})
	}
	message := localizeWith("force_upload_warning",
		"Replace RomM's save with the one on this device?\n{{.Device}}\n{{.Romm}}\nThis ignores which is newer.\nRomM's will no longer be the latest.",
		map[string]any{"Device": deviceSaveLine(local), "Romm": rommLine})
	if !confirmWarning(message) {
		return
	}

	err := whileWorking(localize("force_uploading", "Sending the save to RomM..."), func() error {
		return saves.ForceUpload(client, config, host.DeviceID, local[0])
	})
	if err != nil {
		logger.Error("Failed to upload the save", "game", game.Name, "error", err)
		tell(localize("force_failed", "That did not work.\nSee the Grout logs."))
		return
	}
	logger.Info("Uploaded the device's save by choice", "game", game.Name, "file", local[0].FilePath)
	tell(localize("force_upload_done", "Sent this device's save to RomM."))
}

func forceSlotNote(save *romm.Save) string {
	if slot := saves.SlotOf(save); slot != "autosave" {
		return ", slot '" + slot + "'"
	}
	return ""
}

func deviceSaveLine(local []saves.LocalSave) string {
	if written := saves.NewestWrite(local); !written.IsZero() {
		return localizeWith("force_device_has", "This device: {{.When}}", map[string]any{"When": written.Local().Format("2006-01-02 15:04")})
	}
	return localize("force_device_has_none", "This device has no save.")
}

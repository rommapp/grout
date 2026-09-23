package ui

import (
	"errors"
	"fmt"

	"grout/romm"
	"grout/saves"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	uatomic "go.uber.org/atomic"
)

type SaveSyncInput struct {
	Config        *settings.Config
	Host          settings.Host
	NewSlotName   string           // If set, upload-only mode for a new slot
	NewSlotRomID  int              // ROM ID to upload saves for
	ResolvedItems []saves.SyncItem // If set, skip resolve phase and execute directly
	SessionID     int              // Sync session ID from negotiate (passed through conflict resolution)
}

type SaveSyncOutput struct {
	NeedsConflictResolution bool
	Items                   []saves.SyncItem
	ConflictIndices         map[int]int // maps conflict slice index -> items slice index
	SessionID               int         // Sync session ID to pass through conflict resolution
}

type SaveSyncScreen struct{}

func NewSaveSyncScreen() *SaveSyncScreen {
	return &SaveSyncScreen{}
}

func (s *SaveSyncScreen) Execute(input SaveSyncInput) SaveSyncOutput {
	config := input.Config
	client := romm.NewClientFromHost(input.Host, config.ApiTimeout.Duration())

	// Coming back from the conflict screen, the work is already decided.
	if input.ResolvedItems != nil {
		return s.executeSync(client, config, input.Host.DeviceID, input.ResolvedItems, input.SessionID)
	}

	if _, err := client.GetHeartbeat(); err != nil {
		s.tellUnreachable()
		return SaveSyncOutput{}
	}

	if input.NewSlotName != "" && input.NewSlotRomID > 0 {
		return s.uploadNewSlot(client, config, input.Host.DeviceID, input.NewSlotRomID, input.NewSlotName)
	}

	result, err := s.resolve(client, config, input.Host.DeviceID)
	if err != nil {
		s.tellUnreachable()
		return SaveSyncOutput{}
	}

	items := s.chooseSlots(config, result.Items)

	// Anything the user has to decide goes back to the router for the conflict
	// screen, which returns here with the answers.
	if conflicts := saves.PendingConflicts(items); len(conflicts) > 0 {
		return SaveSyncOutput{
			NeedsConflictResolution: true,
			Items:                   items,
			ConflictIndices:         conflicts,
			SessionID:               result.SessionID,
		}
	}

	return s.executeSync(client, config, input.Host.DeviceID, items, result.SessionID)
}

func (s *SaveSyncScreen) resolve(client *romm.Client, config *settings.Config, deviceID string) (saves.SyncResult, error) {
	var result saves.SyncResult
	var err error

	gaba.ProcessMessage(
		localize("save_sync_scanning", "Scanning saves..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			result, err = saves.ResolveSaveSync(client, config, deviceID)
			return nil, nil
		},
	)

	return result, err
}

func (s *SaveSyncScreen) executeSync(client *romm.Client, config *settings.Config, deviceID string, items []saves.SyncItem, sessionID int) SaveSyncOutput {
	// Executing rewrites each item's action, so what they were has to be
	// noted first.
	watch := saves.WatchUploads(items)

	var report saves.SyncReport
	run := func(progress func(current, total int)) {
		report = saves.ExecuteSaveSync(client, config, deviceID, items, sessionID, progress)
	}

	if saves.Actionable(items) {
		s.withProgress(localize("save_sync_syncing", "Syncing saves..."), run)
	} else {
		run(nil)
	}

	// The server can reject an upload because its own save moved on, which
	// turns that item into a conflict mid-run. Ask about those rather than
	// finishing as though nothing happened.
	if conflicts := watch.Surfaced(report.Items); len(conflicts) > 0 {
		return SaveSyncOutput{
			NeedsConflictResolution: true,
			Items:                   report.Items,
			ConflictIndices:         conflicts,
			SessionID:               sessionID,
		}
	}

	s.showReport(report)
	return SaveSyncOutput{}
}

func (s *SaveSyncScreen) uploadNewSlot(client *romm.Client, config *settings.Config, deviceID string, romID int, slot string) SaveSyncOutput {
	var report saves.SyncReport

	s.withProgress(localize("save_sync_syncing", "Syncing saves..."), func(progress func(current, total int)) {
		items := saves.UploadsForRom(config, romID, slot)
		report = saves.ExecuteSaveSync(client, config, deviceID, items, 0, progress)
	})

	s.showReport(report)
	return SaveSyncOutput{}
}

// withProgress runs work behind a progress bar, handing it a callback to
// report how far along it is.
func (s *SaveSyncScreen) withProgress(message string, work func(progress func(current, total int))) {
	fraction := uatomic.NewFloat64(0)

	gaba.ProcessMessage(
		message,
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
			ShowProgressBar:     true,
			Progress:            fraction,
		},
		func() (any, error) {
			work(func(current, total int) {
				if total > 0 {
					fraction.Store(float64(current) / float64(total))
				}
			})
			return nil, nil
		},
	)
}

// chooseSlots asks which save to take for games the server holds several of.
//
// Backing out skips those downloads rather than quietly pulling whichever slot
// happened to be first: overwriting the wrong save is not recoverable.
func (s *SaveSyncScreen) chooseSlots(config *settings.Config, items []saves.SyncItem) []saves.SyncItem {
	pending := saves.NeedsSlotChoice(items)
	if len(pending) == 0 {
		return items
	}

	rows := make([]gaba.ItemWithOptions, 0, len(pending))
	for _, index := range pending {
		item := items[index]

		options := make([]gaba.Option, 0, len(item.AvailableSlots))
		for _, slot := range item.AvailableSlots {
			// The slot's real name, "autosave" included, so it cannot be
			// mistaken for a server slot actually named "default".
			options = append(options, gaba.Option{DisplayName: slot, Value: slot})
		}

		rows = append(rows, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: fmt.Sprintf("[%s] %s", item.LocalSave.FSSlug, item.LocalSave.RomName)},
			Options:        options,
			SelectedOption: optionIndex(options, config.GetSlotPreference(item.LocalSave.RomID)),
		})
	}

	result, err := gaba.OptionsList(
		localize("game_options_save_slot", "Save Slot"),
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		rows,
	)
	if err != nil {
		if !errors.Is(err, gaba.ErrCancelled) {
			gaba.GetLogger().Warn("Slot selector failed, skipping unconfirmed downloads", "error", err)
		}
		drop := make(map[int]bool, len(pending))
		for _, index := range pending {
			drop[index] = true
		}
		return saves.Without(items, drop)
	}

	for position, index := range pending {
		if position >= len(result.Items) {
			break
		}
		row := result.Items[position]
		if row.SelectedOption < 0 || row.SelectedOption >= len(row.Options) {
			continue
		}
		if slot, ok := row.Options[row.SelectedOption].Value.(string); ok {
			saves.ChooseSlot(config, &items[index], slot)
		}
	}

	if err := settings.SaveSlotPreferences(config); err != nil {
		gaba.GetLogger().Warn("Failed to save slot preferences", "error", err)
	}
	return items
}

func (s *SaveSyncScreen) tellUnreachable() {
	gaba.ConfirmationMessage(
		localize("save_sync_resolve_error", "Failed to connect to server.\nPlease check your connection and try again."),
		ContinueFooter(),
		gaba.MessageOptions{},
	)
}

func (s *SaveSyncScreen) showReport(report saves.SyncReport) {
	sections := reportSections(report)
	if len(sections) == 0 {
		gaba.ConfirmationMessage(
			localize("save_sync_no_changes", "Everything is up to date.\nGo play some games!"),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ConfirmButton = buttons.VirtualButtonA

	gaba.DetailScreen(localize("save_sync_results_title", "Sync Complete"), options, ContinueFooter())
}

// reportGroups are the outcomes worth reporting, in the order they are shown.
//
// count comes from the tally the sync itself kept and includes says which
// items belong to it. The two are written down together because a header
// saying three over an empty list is how they go wrong apart.
var reportGroups = []struct {
	id, fallback string
	count        func(saves.SyncReport) int
	includes     func(saves.SyncItem) bool
}{
	{
		"save_sync_uploaded", "Uploaded",
		func(r saves.SyncReport) int { return r.Uploaded },
		func(i saves.SyncItem) bool { return i.Action == saves.ActionUpload && i.Success },
	},
	{
		"save_sync_downloaded", "Downloaded",
		func(r saves.SyncReport) int { return r.Downloaded },
		func(i saves.SyncItem) bool { return i.Action == saves.ActionDownload && i.Success },
	},
	{
		"save_sync_conflicts", "Conflicts",
		func(r saves.SyncReport) int { return r.Conflicts },
		func(i saves.SyncItem) bool { return i.Action == saves.ActionConflict },
	},
	{
		"save_sync_errors", "Errors",
		func(r saves.SyncReport) int { return r.Errors },
		func(i saves.SyncItem) bool {
			return (i.Action == saves.ActionUpload || i.Action == saves.ActionDownload) && !i.Success
		},
	},
}

func reportSections(report saves.SyncReport) []gaba.Section {
	var sections []gaba.Section

	for _, group := range reportGroups {
		count := group.count(report)
		if count == 0 {
			continue
		}

		var rows []gaba.MetadataItem
		for _, item := range report.Items {
			if group.includes(item) {
				rows = append(rows, gaba.MetadataItem{Label: item.LocalSave.RomName})
			}
		}

		sections = append(sections, gaba.NewInfoSection(
			fmt.Sprintf("%s (%d)", localize(group.id, group.fallback), count), rows))
	}

	return sections
}

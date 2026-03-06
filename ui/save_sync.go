package ui

import (
	"fmt"
	"grout/internal"
	"grout/romm"
	"grout/sync"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	uatomic "go.uber.org/atomic"
)

type SaveSyncInput struct {
	Config        *internal.Config
	Host          romm.Host
	NewSlotName   string          // If set, upload-only mode for a new slot
	NewSlotRomID  int             // ROM ID to upload saves for
	ResolvedItems []sync.SyncItem // If set, skip resolve phase and execute directly
}

type SaveSyncOutput struct {
	NeedsConflictResolution bool
	Items                   []sync.SyncItem
	ConflictIndices         map[int]int // maps conflict slice index → items slice index
}

type SaveSyncScreen struct{}

func NewSaveSyncScreen() *SaveSyncScreen {
	return &SaveSyncScreen{}
}

func (s *SaveSyncScreen) Execute(input SaveSyncInput) SaveSyncOutput {
	config := input.Config
	host := input.Host
	client := romm.NewClientFromHost(host, config.ApiTimeout)

	// If we have resolved items from the conflict screen, skip to execute phase
	if input.ResolvedItems != nil {
		return s.executeSyncPhase(client, config, host.DeviceID, input.ResolvedItems)
	}

	// Health check — verify server is reachable before starting sync
	if _, err := client.GetHeartbeat(); err != nil {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "save_sync_resolve_error", Other: "Failed to connect to server.\nPlease check your connection and try again."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return SaveSyncOutput{}
	}

	// New slot upload: skip resolve, scan local saves and upload to the new slot
	if input.NewSlotName != "" && input.NewSlotRomID > 0 {
		return s.executeNewSlotUpload(client, config, host.DeviceID, input.NewSlotRomID, input.NewSlotName)
	}

	// Phase 1: Resolve — scan local saves, fetch summaries, determine actions
	var items []sync.SyncItem
	var resolveErr error
	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "save_sync_scanning", Other: "Scanning saves..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			var err error
			items, err = sync.ResolveSaveSync(client, config, host.DeviceID)
			resolveErr = err
			return nil, nil
		},
	)

	if resolveErr != nil {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "save_sync_resolve_error", Other: "Failed to connect to server.\nPlease check your connection and try again."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return SaveSyncOutput{}
	}

	// Slot selection for first-time downloads with multiple slots
	items = s.resolveMultiSlotDownloads(config, items)

	// Check for conflicts — if any, return to router for conflict screen
	conflictIndices := map[int]int{} // maps conflict slice index → items slice index
	hasConflicts := false
	conflictCount := 0
	for i, item := range items {
		if item.Action == sync.ActionConflict {
			conflictIndices[conflictCount] = i
			conflictCount++
			hasConflicts = true
		}
	}

	if hasConflicts {
		return SaveSyncOutput{
			NeedsConflictResolution: true,
			Items:                   items,
			ConflictIndices:         conflictIndices,
		}
	}

	// No conflicts — execute directly
	return s.executeSyncPhase(client, config, host.DeviceID, items)
}

func (s *SaveSyncScreen) executeSyncPhase(client *romm.Client, config *internal.Config, deviceID string, items []sync.SyncItem) SaveSyncOutput {
	var report sync.SyncReport

	hasActionable := false
	for _, item := range items {
		if item.Action != sync.ActionSkip && item.Action != sync.ActionConflict {
			hasActionable = true
			break
		}
	}

	if hasActionable {
		progress := uatomic.NewFloat64(0)
		gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "save_sync_syncing", Other: "Syncing saves..."}, nil),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (any, error) {
				report = sync.ExecuteSaveSync(client, config, deviceID, items, func(current, total int) {
					if total > 0 {
						progress.Store(float64(current) / float64(total))
					}
				})
				return nil, nil
			},
		)
	} else {
		report = sync.ExecuteSaveSync(client, config, deviceID, items, nil)
	}

	s.showReport(report)
	return SaveSyncOutput{}
}

func (s *SaveSyncScreen) executeNewSlotUpload(client *romm.Client, config *internal.Config, deviceID string, romID int, slotName string) SaveSyncOutput {
	var report sync.SyncReport
	progress := uatomic.NewFloat64(0)

	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "save_sync_syncing", Other: "Syncing saves..."}, nil),
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
			ShowProgressBar:     true,
			Progress:            progress,
		},
		func() (any, error) {
			localSaves := sync.ScanSaves(config)
			var items []sync.SyncItem
			for _, ls := range localSaves {
				if ls.RomID == romID {
					items = append(items, sync.SyncItem{
						LocalSave:  ls,
						TargetSlot: slotName,
						Action:     sync.ActionUpload,
					})
				}
			}
			report = sync.ExecuteSaveSync(client, config, deviceID, items, func(current, total int) {
				if total > 0 {
					progress.Store(float64(current) / float64(total))
				}
			})
			return nil, nil
		},
	)

	s.showReport(report)
	return SaveSyncOutput{}
}

// resolveMultiSlotDownloads shows a slot picker for first-time downloads that have
// multiple slots on the server. Returns the (potentially modified) items slice.
func (s *SaveSyncScreen) resolveMultiSlotDownloads(config *internal.Config, items []sync.SyncItem) []sync.SyncItem {
	defaultLabel := i18n.Localize(&goi18n.Message{ID: "common_default", Other: "Default"}, nil)

	// Collect items that need slot selection
	type slotChoice struct {
		itemIndex int
		romName   string
		slots     []string
	}
	var choices []slotChoice
	for i, item := range items {
		if len(item.AvailableSlots) > 1 {
			choices = append(choices, slotChoice{
				itemIndex: i,
				romName:   item.LocalSave.RomName,
				slots:     item.AvailableSlots,
			})
		}
	}

	if len(choices) == 0 {
		return items
	}

	// Build an OptionsList with one row per game
	optionItems := make([]gaba.ItemWithOptions, 0, len(choices))
	for _, c := range choices {
		options := make([]gaba.Option, 0, len(c.slots))
		for _, slot := range c.slots {
			displayName := slot
			if slot == "default" {
				displayName = defaultLabel
			}
			options = append(options, gaba.Option{DisplayName: displayName, Value: slot})
		}

		currentPref := config.GetSlotPreference(items[c.itemIndex].LocalSave.RomID)
		selectedIdx := 0
		for i, slot := range c.slots {
			if slot == currentPref {
				selectedIdx = i
				break
			}
		}

		displayText := fmt.Sprintf("[%s] %s", items[c.itemIndex].LocalSave.FSSlug, c.romName)
		optionItems = append(optionItems, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: displayText},
			Options:        options,
			SelectedOption: selectedIdx,
		})
	}

	saveSlotText := i18n.Localize(&goi18n.Message{ID: "game_options_save_slot", Other: "Save Slot"}, nil)

	result, err := gaba.OptionsList(
		saveSlotText,
		gaba.OptionListSettings{
			FooterHelpItems:      OptionsListFooter(),
			InitialSelectedIndex: 0,
			StatusBar:            StatusBar(),
			UseSmallTitle:        true,
		},
		optionItems,
	)
	if err != nil {
		return items // Cancelled — proceed with defaults
	}

	// Apply selections
	for ci, c := range choices {
		if ci < len(result.Items) {
			item := result.Items[ci]
			if item.SelectedOption >= 0 && item.SelectedOption < len(item.Options) {
				if selectedSlot, ok := item.Options[item.SelectedOption].Value.(string); ok {
					config.SetSlotPreference(items[c.itemIndex].LocalSave.RomID, selectedSlot)
					newSave := sync.SelectSaveForSlot(items[c.itemIndex].AllRemoteSaves, selectedSlot)
					if newSave != nil {
						items[c.itemIndex].RemoteSave = newSave
					}
				}
			}
		}
	}

	if err := internal.SaveSlotPreferences(config); err != nil {
		gaba.GetLogger().Warn("Failed to save slot preferences", "error", err)
	}
	return items
}

func (s *SaveSyncScreen) showReport(report sync.SyncReport) {
	sections := s.buildReportSections(report)

	if len(sections) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "save_sync_no_changes", Other: "Everything is up to date.\nGo play some games!"}, nil),
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

	gaba.DetailScreen(
		i18n.Localize(&goi18n.Message{ID: "save_sync_results_title", Other: "Sync Complete"}, nil),
		options,
		ContinueFooter(),
	)
}

func (s *SaveSyncScreen) buildReportSections(report sync.SyncReport) []gaba.Section {
	var sections []gaba.Section

	if report.Uploaded > 0 {
		var items []gaba.MetadataItem
		for _, item := range report.Items {
			if item.Action == sync.ActionUpload && item.Success {
				items = append(items, gaba.MetadataItem{
					Label: item.LocalSave.RomName,
				})
			}
		}
		sections = append(sections, gaba.NewInfoSection(
			fmt.Sprintf("%s (%d)", i18n.Localize(&goi18n.Message{ID: "save_sync_uploaded", Other: "Uploaded"}, nil), report.Uploaded),
			items,
		))
	}

	if report.Downloaded > 0 {
		var items []gaba.MetadataItem
		for _, item := range report.Items {
			if item.Action == sync.ActionDownload && item.Success {
				items = append(items, gaba.MetadataItem{
					Label: item.LocalSave.RomName,
				})
			}
		}
		sections = append(sections, gaba.NewInfoSection(
			fmt.Sprintf("%s (%d)", i18n.Localize(&goi18n.Message{ID: "save_sync_downloaded", Other: "Downloaded"}, nil), report.Downloaded),
			items,
		))
	}

	if report.Conflicts > 0 {
		var items []gaba.MetadataItem
		for _, item := range report.Items {
			if item.Action == sync.ActionConflict {
				items = append(items, gaba.MetadataItem{
					Label: item.LocalSave.RomName,
				})
			}
		}
		sections = append(sections, gaba.NewInfoSection(
			fmt.Sprintf("%s (%d)", i18n.Localize(&goi18n.Message{ID: "save_sync_conflicts", Other: "Conflicts"}, nil), report.Conflicts),
			items,
		))
	}

	if report.Errors > 0 {
		var items []gaba.MetadataItem
		for _, item := range report.Items {
			if (item.Action == sync.ActionUpload || item.Action == sync.ActionDownload) && !item.Success {
				items = append(items, gaba.MetadataItem{
					Label: item.LocalSave.RomName,
				})
			}
		}
		sections = append(sections, gaba.NewInfoSection(
			fmt.Sprintf("%s (%d)", i18n.Localize(&goi18n.Message{ID: "save_sync_errors", Other: "Errors"}, nil), report.Errors),
			items,
		))
	}

	return sections
}

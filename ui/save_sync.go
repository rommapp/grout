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
	Config *internal.Config
	Host   romm.Host
}

type SaveSyncOutput struct{}

type SaveSyncScreen struct{}

func NewSaveSyncScreen() *SaveSyncScreen {
	return &SaveSyncScreen{}
}

func (s *SaveSyncScreen) Execute(config *internal.Config, host romm.Host) SaveSyncOutput {
	client := romm.NewClientFromHost(host, config.ApiTimeout)

	// Phase 1: Resolve — scan local saves, fetch summaries, determine actions
	var items []sync.SyncItem
	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "save_sync_scanning", Other: "Scanning saves..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			var err error
			items, err = sync.ResolveSaveSync(client, config, host.DeviceID)
			return nil, err
		},
	)

	// Check for conflicts and show resolution screen
	var conflicts []sync.SyncItem
	conflictIndices := map[int]int{} // maps conflict slice index → items slice index
	for i, item := range items {
		if item.Action == sync.ActionConflict {
			conflictIndices[len(conflicts)] = i
			conflicts = append(conflicts, item)
		}
	}

	if len(conflicts) > 0 {
		conflictScreen := NewSaveConflictScreen()
		result, err := conflictScreen.Draw(SaveConflictInput{Items: conflicts})
		if err == nil && result.Action == SaveConflictActionResolved {
			for ci, resolved := range result.Items {
				if idx, ok := conflictIndices[ci]; ok {
					items[idx].Action = resolved.Action
				}
			}
		}
	}

	// Phase 2: Execute — upload/download based on resolved actions
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
				report = sync.ExecuteSaveSync(client, config, host.DeviceID, items, func(current, total int) {
					if total > 0 {
						progress.Store(float64(current) / float64(total))
					}
				})
				return nil, nil
			},
		)
	} else {
		report = sync.ExecuteSaveSync(client, config, host.DeviceID, items, nil)
	}

	s.showReport(report)

	return SaveSyncOutput{}
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
				fileName := item.LocalSave.FileName
				if fileName == "" && item.RemoteSave != nil {
					fileName = item.RemoteSave.FileName
				}
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

package ui

import (
	"errors"
	"fmt"
	"grout/sync"
	"path/filepath"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type syncReportInput struct {
	Results   []sync.SyncResult
	Unmatched []sync.UnmatchedSave
}

type syncReportOutput struct{}

type SyncReportScreen struct{}

func newSyncReportScreen() *SyncReportScreen {
	return &SyncReportScreen{}
}

func (s *SyncReportScreen) draw(input syncReportInput) (ScreenResult[syncReportOutput], error) {
	logger := gaba.GetLogger()
	output := syncReportOutput{}

	sections := s.buildSections(input.Results, input.Unmatched)

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true

	result, err := gaba.DetailScreen(i18n.Localize(&goi18n.Message{ID: "save_sync_summary", Other: "Save Sync Summary"}, nil), options, []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_close", Other: "Close"}, nil)},
	})

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(output), nil
		}
		logger.Error("Detail screen error", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	if result.Action == gaba.DetailActionCancelled {
		return back(output), nil
	}

	return success(output), nil
}

func (s *SyncReportScreen) buildSections(results []sync.SyncResult, unmatched []sync.UnmatchedSave) []gaba.Section {
	logger := gaba.GetLogger()
	logger.Debug("Building sync report", "totalResults", len(results), "unmatched", len(unmatched))

	sections := make([]gaba.Section, 0)

	uploadedCount := 0
	downloadedCount := 0
	skippedCount := 0
	failedCount := 0

	for _, r := range results {
		if !r.Success {
			failedCount++
			continue
		}
		switch r.Action {
		case sync.Upload:
			uploadedCount++
		case sync.Download:
			downloadedCount++
		case sync.Skip:
			skippedCount++
		}
	}

	summary := []gaba.MetadataItem{
		{Label: i18n.Localize(&goi18n.Message{ID: "save_sync_total_processed", Other: "Total Processed"}, nil), Value: fmt.Sprintf("%d", len(results))},
	}

	if downloadedCount > 0 {
		summary = append(summary, gaba.MetadataItem{Label: i18n.Localize(&goi18n.Message{ID: "save_sync_downloaded", Other: "Downloaded"}, nil), Value: fmt.Sprintf("%d", downloadedCount)})
	}

	if uploadedCount > 0 {
		summary = append(summary, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "save_sync_uploaded", Other: "Uploaded"}, nil), Value: fmt.Sprintf("%d", uploadedCount)})
	}

	if skippedCount > 0 {
		summary = append(summary, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "save_sync_skipped", Other: "Skipped"}, nil), Value: fmt.Sprintf("%d", skippedCount)})
	}

	if failedCount > 0 {
		summary = append(summary, gaba.MetadataItem{
			Label: i18n.Localize(&goi18n.Message{ID: "save_sync_failed", Other: "Failed"}, nil), Value: fmt.Sprintf("%d", failedCount)})
	}

	sections = append(sections, gaba.NewInfoSection(i18n.Localize(&goi18n.Message{ID: "save_sync_summary_section", Other: "Summary"}, nil), summary))

	if downloadedCount > 0 {
		downloadedFiles := ""
		for _, r := range results {
			if r.Success && r.Action == sync.Download {
				if downloadedFiles != "" {
					downloadedFiles += "\n"
				}
				displayName := r.RomDisplayName
				if displayName == "" {
					displayName = filepath.Base(r.FilePath)
				}
				downloadedFiles += displayName
			}
		}
		sections = append(sections, gaba.NewDescriptionSection(i18n.Localize(&goi18n.Message{ID: "save_sync_downloaded", Other: "Downloaded"}, nil), downloadedFiles))
	}

	if uploadedCount > 0 {
		uploadedFiles := ""
		for _, r := range results {
			if r.Success && r.Action == sync.Upload {
				if uploadedFiles != "" {
					uploadedFiles += "\n"
				}
				displayName := r.RomDisplayName
				if displayName == "" {
					displayName = filepath.Base(r.FilePath)
				}
				logger.Debug("Upload result for report",
					"gameName", r.GameName,
					"romDisplayName", r.RomDisplayName,
					"filePath", r.FilePath,
					"displayName", displayName)
				uploadedFiles += displayName
			}
		}
		sections = append(sections, gaba.NewDescriptionSection(i18n.Localize(&goi18n.Message{ID: "save_sync_uploaded", Other: "Uploaded"}, nil), uploadedFiles))
	}

	if failedCount > 0 {
		failedFiles := ""
		for _, r := range results {
			if !r.Success {
				if failedFiles != "" {
					failedFiles += "\n"
				}
				errorMsg := r.Error
				if errorMsg == "" {
					errorMsg = i18n.Localize(&goi18n.Message{ID: "save_sync_unknown_error", Other: "Unknown error"}, nil)
				}
				// Check for orphan ROM error and provide localized message
				if errors.Is(r.Err, sync.ErrOrphanRom) {
					errorMsg = i18n.Localize(&goi18n.Message{ID: "save_sync_orphan_rom_error", Other: "ROM not matched. Use 'Match Orphans By Hash' in Advanced Settings"}, nil)
				}
				displayName := r.RomDisplayName
				if displayName == "" {
					displayName = r.GameName
				}
				failedFiles += fmt.Sprintf("%s (%s): %s", displayName, r.Action, errorMsg)
			}
		}
		sections = append(sections, gaba.NewDescriptionSection(i18n.Localize(&goi18n.Message{ID: "save_sync_failed", Other: "Failed"}, nil), failedFiles))
	}

	// Display unmatched saves (ROM not found in RomM)
	if len(unmatched) > 0 {
		unmatchedText := ""
		for _, u := range unmatched {
			if unmatchedText != "" {
				unmatchedText += "\n"
			}
			unmatchedText += i18n.Localize(&goi18n.Message{ID: "save_sync_rom_not_found", Other: "{{.Name}} (ROM not found in RomM)"}, map[string]interface{}{"Name": filepath.Base(u.SavePath)})
		}
		sections = append(sections, gaba.NewDescriptionSection(i18n.Localize(&goi18n.Message{ID: "save_sync_unmatched_saves", Other: "Unmatched Saves"}, nil), unmatchedText))
	}

	return sections
}

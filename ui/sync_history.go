package ui

import (
	"grout/saves"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type SyncHistoryInput struct {
	DeviceID string
}

type SyncHistoryOutput struct {
	Action SyncHistoryAction
}

type SyncHistoryScreen struct{}

func NewSyncHistoryScreen() *SyncHistoryScreen {
	return &SyncHistoryScreen{}
}

func (s *SyncHistoryScreen) Draw(input SyncHistoryInput) (SyncHistoryOutput, error) {
	output := SyncHistoryOutput{Action: SyncHistoryActionBack}

	days := saves.SyncHistory(input.DeviceID)
	if len(days) == 0 {
		gaba.ConfirmationMessage(
			localize("sync_history_empty", "No sync history found."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = historySections(days)
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ConfirmButton = buttons.VirtualButtonUnassigned

	gaba.DetailScreen(
		localize("sync_history_title", "Sync History"),
		options,
		[]gaba.FooterHelpItem{FooterBack()},
	)

	return output, nil
}

// The icons are Material Design codepoints in the toolkit's font. The plain
// cloud heads the column that says which way each save went.
const (
	cloudOutline         = "\U000F0163"
	cloudDownloadOutline = "\U000F0B7D"
	cloudUploadOutline   = "\U000F0B7E"
)

// actionIcon shows which way a save went. An action grout does not recognise
// is written out rather than dropped, so a new one shows up as itself instead
// of as a blank cell.
func actionIcon(action string) string {
	switch action {
	case "upload":
		return cloudUploadOutline
	case "download":
		return cloudDownloadOutline
	default:
		return action
	}
}

func historySections(days []saves.SyncDay) []gaba.Section {
	headers := []string{
		cloudOutline,
		localize("sync_history_col_game", "Game"),
		localize("sync_history_col_platform", "Platform"),
		localize("sync_history_col_time", "Time"),
	}

	sections := make([]gaba.Section, 0, len(days))
	for _, day := range days {
		rows := make([]gaba.TableRow, len(day.Events))
		for i, event := range day.Events {
			rows[i] = gaba.TableRow{Cells: []string{
				actionIcon(event.Action),
				event.RomName,
				event.Platform,
				event.At.Format("15:04"),
			}}
		}

		sections = append(sections, gaba.NewTableSection(
			day.Date.Format("January 2, 2006"), headers, rows, gaba.TableGridRowDividers))
	}

	return sections
}

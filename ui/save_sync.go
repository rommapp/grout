package ui

import (
	"grout/romm"
	"grout/utils"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SaveSyncInput struct {
	Config *utils.Config
	Host   romm.Host
}

type SaveSyncOutput struct{}

type SaveSyncScreen struct{}

func NewSaveSyncScreen() *SaveSyncScreen {
	return &SaveSyncScreen{}
}

func (s *SaveSyncScreen) Draw(input SaveSyncInput) (ScreenResult[SaveSyncOutput], error) {
	output := SaveSyncOutput{}

	type scanResult struct {
		Syncs     []utils.SaveSync
		Unmatched []utils.UnmatchedSave
	}

	scanData, _ := gaba.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "save_sync_scanning", Other: "Scanning save files..."}, nil), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		syncs, unmatched, err := utils.FindSaveSyncs(input.Host)
		if err != nil {
			gaba.GetLogger().Error("Unable to scan save files!", "error", err)
			return nil, nil
		}

		return scanResult{Syncs: syncs, Unmatched: unmatched}, nil
	})

	var results []utils.SyncResult
	var unmatched []utils.UnmatchedSave

	if scan, ok := scanData.(scanResult); ok {
		unmatched = scan.Unmatched
		results = make([]utils.SyncResult, 0, len(scan.Syncs))

		for i := range scan.Syncs {
			s := &scan.Syncs[i]
			result := s.Execute(input.Host, input.Config)
			results = append(results, result)
			if !result.Success {
				gaba.GetLogger().Error("Unable to sync save!", "game", s.GameBase, "error", result.Error)
			}
		}
	}

	if len(results) > 0 || len(unmatched) > 0 {
		reportScreen := newSyncReportScreen()
		_, err := reportScreen.draw(syncReportInput{
			Results:   results,
			Unmatched: unmatched,
		})
		if err != nil {
			gaba.GetLogger().Error("Error showing sync report", "error", err)
		}
	} else {
		gaba.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "save_sync_up_to_date", Other: "Everything is up to date!\nGo play some games!"}, nil), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
			time.Sleep(time.Second * 2)
			return nil, nil
		})
	}

	return back(output), nil
}

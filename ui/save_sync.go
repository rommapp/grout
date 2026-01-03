package ui

import (
	"grout/romm"
	"grout/utils"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
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

	// Scan local ROMs and match with save files
	romScan, _ := gaba.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "save_sync_scanning_roms", Other: "Scanning ROMs..."}, nil), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		return utils.ScanRoms(), nil
	})

	type scanResult struct {
		Syncs     []utils.SaveSync
		Unmatched []utils.UnmatchedSave
	}

	// Then, find save syncs using the pre-scanned ROM data
	scanData, _ := gaba.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "save_sync_scanning", Other: "Scanning save files..."}, nil), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		localRoms, ok := romScan.(utils.LocalRomScan)
		if !ok {
			gaba.GetLogger().Error("Unable to scan ROMs!")
			return nil, nil
		}

		syncs, unmatched, err := utils.FindSaveSyncsFromScan(input.Host, localRoms)
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

		if len(scan.Syncs) > 0 {
			progress := &atomic.Float64{}

			gaba.ProcessMessage(
				i18n.Localize(&goi18n.Message{ID: "save_sync_syncing", Other: "Syncing saves..."}, nil),
				gaba.ProcessMessageOptions{
					ShowProgressBar: true,
					Progress:        progress,
				},
				func() (interface{}, error) {
					total := len(scan.Syncs)
					for i := range scan.Syncs {
						s := &scan.Syncs[i]
						result := s.Execute(input.Host, input.Config)
						results = append(results, result)
						if !result.Success {
							gaba.GetLogger().Error("Unable to sync save!", "game", s.GameBase, "error", result.Error)
						}
						progress.Store(float64(i+1) / float64(total))
					}
					return nil, nil
				},
			)
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

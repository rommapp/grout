package ui

import (
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/romm"
	"grout/sync"
	"os"
	"path/filepath"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"go.uber.org/atomic"
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

func (s *SaveSyncScreen) Draw(input SaveSyncInput) (SaveSyncOutput, error) {
	output := SaveSyncOutput{}
	config := input.Config

	romScan, _ := gaba.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "save_sync_scanning_roms", Other: "Scanning ROMs..."}, nil), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		return sync.ScanRoms(config), nil
	})

	type scanResult struct {
		Syncs        []sync.SaveSync
		Unmatched    []sync.UnmatchedSave
		FuzzyMatches []sync.PendingFuzzyMatch
	}

	scanData, _ := gaba.ProcessMessage(i18n.Localize(&goi18n.Message{ID: "save_sync_scanning", Other: "Scanning save files..."}, nil), gaba.ProcessMessageOptions{}, func() (interface{}, error) {
		localRoms, ok := romScan.(sync.LocalRomScan)
		if !ok {
			gaba.GetLogger().Error("Unable to scan ROMs!")
			return nil, nil
		}

		syncs, unmatched, fuzzyMatches, err := sync.FindSaveSyncsFromScan(input.Host, input.Config, localRoms)
		if err != nil {
			gaba.GetLogger().Error("Unable to scan save files!", "error", err)
			return nil, nil
		}

		return scanResult{Syncs: syncs, Unmatched: unmatched, FuzzyMatches: fuzzyMatches}, nil
	})

	var results []sync.Result
	var unmatched []sync.UnmatchedSave

	if scan, ok := scanData.(scanResult); ok {
		unmatched = scan.Unmatched
		syncs := scan.Syncs

		for _, fm := range scan.FuzzyMatches {
			confirmed := showFuzzyMatchConfirmation(fm)
			if confirmed {
				err := cache.SaveFilenameMapping(fm.FSSlug, fm.LocalFilename, fm.MatchedRomID, fm.MatchedName)
				if err != nil {
					gaba.GetLogger().Error("Failed to save filename mapping", "error", err)
				} else {
					_ = cache.ClearFailedLookup(fm.FSSlug, fm.LocalFilename)
					gaba.GetLogger().Info("Fuzzy match confirmed and saved",
						"local", fm.LocalFilename,
						"matched", fm.MatchedName)

					if localSave := createLocalSaveFromPath(fm.SavePath, fm.FSSlug); localSave != nil {
						gameBase := strings.TrimSuffix(filepath.Base(fm.SavePath), filepath.Ext(fm.SavePath))
						syncs = append(syncs, sync.SaveSync{
							RomID:    fm.MatchedRomID,
							RomName:  fm.MatchedName,
							FSSlug:   fm.FSSlug,
							GameBase: gameBase,
							Local:    localSave,
							Action:   sync.Upload,
						})
					}
				}
			} else {
				_ = cache.RecordFailedLookup(fm.FSSlug, fm.LocalFilename)
				unmatched = append(unmatched, sync.UnmatchedSave{
					SavePath: fm.SavePath,
					FSSlug:   fm.FSSlug,
				})
			}
		}

		results = make([]sync.Result, 0, len(syncs))

		if len(syncs) > 0 {
			progress := &atomic.Float64{}

			gaba.ProcessMessage(
				i18n.Localize(&goi18n.Message{ID: "save_sync_syncing", Other: "Syncing saves..."}, nil),
				gaba.ProcessMessageOptions{
					ShowProgressBar: true,
					Progress:        progress,
				},
				func() (interface{}, error) {
					total := len(syncs)
					for i := range syncs {
						s := &syncs[i]
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

	return output, nil
}

func showFuzzyMatchConfirmation(fm sync.PendingFuzzyMatch) bool {
	similarityPercent := int(fm.Similarity * 100)
	message := fmt.Sprintf("%s\n\n%s\n%s\n%s\n\n%s",
		i18n.Localize(&goi18n.Message{
			ID:    "fuzzy_match_title",
			Other: "Potential Match Found",
		}, nil),
		i18n.Localize(&goi18n.Message{
			ID:    "fuzzy_match_local",
			Other: "Local: \"{{.Name}}\"",
		}, map[string]interface{}{"Name": fm.LocalFilename}),
		i18n.Localize(&goi18n.Message{
			ID:    "fuzzy_match_remote",
			Other: "Match: \"{{.Name}}\"",
		}, map[string]interface{}{"Name": fm.MatchedName}),
		i18n.Localize(&goi18n.Message{
			ID:    "fuzzy_match_similarity",
			Other: "Similarity: {{.Percent}}%",
		}, map[string]interface{}{"Percent": similarityPercent}),
		i18n.Localize(&goi18n.Message{
			ID:    "fuzzy_match_confirm",
			Other: "Is this the same game?",
		}, nil),
	)

	_, err := gaba.ConfirmationMessage(
		message,
		[]gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "fuzzy_match_no", Other: "No"}, nil)},
			{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "fuzzy_match_yes", Other: "Yes"}, nil)},
		},
		gaba.MessageOptions{
			ConfirmButton: buttons.VirtualButtonX,
		},
	)

	return err == nil
}

func createLocalSaveFromPath(savePath, fsSlug string) *sync.LocalSave {
	info, err := os.Stat(savePath)
	if err != nil {
		gaba.GetLogger().Error("Failed to stat save file", "path", savePath, "error", err)
		return nil
	}

	return &sync.LocalSave{
		FSSlug:       fsSlug,
		Path:         savePath,
		LastModified: info.ModTime(),
	}
}

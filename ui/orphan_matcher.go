package ui

import (
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/romm"
	"grout/sync"
	"sync/atomic"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type OrphanMatcherInput struct {
	Config internal.Config
	Host   romm.Host
}

type OrphanMatcherOutput struct{}

type OrphanMatcherScreen struct{}

func NewOrphanMatcherScreen() *OrphanMatcherScreen {
	return &OrphanMatcherScreen{}
}

func (s *OrphanMatcherScreen) Execute(config internal.Config, host romm.Host) OrphanMatcherOutput {
	s.draw(OrphanMatcherInput{
		Config: config,
		Host:   host,
	})
	return OrphanMatcherOutput{}
}

func (s *OrphanMatcherScreen) draw(input OrphanMatcherInput) {
	logger := gaba.GetLogger()

	cm := cache.GetCacheManager()
	if cm == nil {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "orphan_match_no_cache", Other: "Cache not available."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	// Scan local ROMs
	var localRoms sync.LocalRomScan
	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "orphan_match_scanning", Other: "Scanning local ROMs..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			localRoms = sync.ScanRoms()
			return nil, nil
		},
	)

	if len(localRoms) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "orphan_match_no_roms", Other: "No local ROMs found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	// Find orphan ROMs (not in cache)
	type orphanRom struct {
		fsSlug   string
		fileName string
		filePath string
	}
	var orphans []orphanRom

	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "orphan_match_finding", Other: "Finding orphan ROMs..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			for fsSlug, roms := range localRoms {
				for _, rom := range roms {
					// Check if ROM is already in cache
					romID, _, found := cache.GetCachedRomIDByFilename(fsSlug, rom.FileName)
					if !found || romID == 0 {
						orphans = append(orphans, orphanRom{
							fsSlug:   fsSlug,
							fileName: rom.FileName,
							filePath: rom.FilePath,
						})
					}
				}
			}
			return nil, nil
		},
	)

	if len(orphans) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "orphan_match_none_found", Other: "No orphan ROMs found. All ROMs are already matched!"}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	// Show confirmation before processing
	_, err := gaba.ConfirmationMessage(
		fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "orphan_match_confirm", Other: "Found %d orphan ROMs. Match by hash?"}, nil), len(orphans)),
		[]gaba.FooterHelpItem{
			FooterCancel(),
			FooterConfirm(),
		},
		gaba.MessageOptions{},
	)

	if err != nil {
		// User cancelled
		return
	}

	// Process orphans - compute hash and look up in RomM
	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout)
	var matchedCount int32
	var processedCount int32
	total := len(orphans)

	for idx, orphan := range orphans {
		// Show progress
		gaba.ProcessMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "orphan_match_processing", Other: "Processing %d/%d: %s"}, nil), idx+1, total, orphan.fileName),
			gaba.ProcessMessageOptions{ShowThemeBackground: true},
			func() (interface{}, error) {
				atomic.AddInt32(&processedCount, 1)

				// Skip if file path is empty
				if orphan.filePath == "" {
					logger.Debug("Skipping orphan with empty path", "fileName", orphan.fileName)
					return nil, nil
				}

				// Compute CRC32 hash
				crcHash, err := fileutil.ComputeCRC32(orphan.filePath)
				if err != nil {
					logger.Debug("Failed to compute CRC32 hash", "file", orphan.fileName, "error", err)
					return nil, nil
				}

				logger.Debug("Looking up ROM by CRC32 hash", "file", orphan.fileName, "crc", crcHash)

				// Look up in RomM by CRC32 hash first
				rom, err := client.GetRomByHash(romm.GetRomByHashQuery{CrcHash: crcHash})
				if err != nil {
					logger.Debug("CRC32 lookup failed", "file", orphan.fileName, "crc", crcHash, "error", err)
				}

				// If CRC32 didn't find a match, try SHA1 as fallback
				if rom.ID == 0 {
					sha1Hash, err := fileutil.ComputeSHA1(orphan.filePath)
					if err != nil {
						logger.Debug("Failed to compute SHA1 hash", "file", orphan.fileName, "error", err)
						return nil, nil
					}

					logger.Debug("Looking up ROM by SHA1 hash", "file", orphan.fileName, "sha1", sha1Hash)

					rom, err = client.GetRomByHash(romm.GetRomByHashQuery{Sha1Hash: sha1Hash})
					if err != nil {
						logger.Debug("SHA1 lookup failed", "file", orphan.fileName, "sha1", sha1Hash, "error", err)
						return nil, nil
					}

					if rom.ID == 0 {
						logger.Debug("No ROM matched for either hash", "file", orphan.fileName, "crc", crcHash, "sha1", sha1Hash)
						return nil, nil
					}

					logger.Info("Matched orphan ROM by SHA1 hash",
						"file", orphan.fileName,
						"sha1", sha1Hash,
						"romID", rom.ID,
						"romName", rom.Name)
				} else {
					logger.Info("Matched orphan ROM by CRC32 hash",
						"file", orphan.fileName,
						"crc", crcHash,
						"romID", rom.ID,
						"romName", rom.Name)
				}

				if err := cache.SaveFilenameMapping(orphan.fsSlug, orphan.fileName, rom.ID, rom.Name); err != nil {
					logger.Warn("Failed to save filename mapping", "romID", rom.ID, "error", err)
					return nil, nil
				}

				atomic.AddInt32(&matchedCount, 1)
				return nil, nil
			},
		)
	}

	finalMatched := int(atomic.LoadInt32(&matchedCount))
	logger.Info("Orphan matching complete", "matched", finalMatched, "total", total)

	// Show completion message
	if finalMatched > 0 {
		gaba.ConfirmationMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "orphan_match_complete", Other: "Matched %d of %d orphan ROMs."}, nil), finalMatched, total),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	} else {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "orphan_match_no_matches", Other: "No orphan ROMs could be matched. They may not exist in your RomM library."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	}
}

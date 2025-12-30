package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"grout/constants"
	"grout/romm"
	"grout/utils"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type BIOSDownloadInput struct {
	Config   utils.Config
	Host     romm.Host
	Platform romm.Platform
}

type BIOSDownloadOutput struct {
	Platform romm.Platform
}

type BIOSDownloadScreen struct{}

func NewBIOSDownloadScreen() *BIOSDownloadScreen {
	return &BIOSDownloadScreen{}
}

func (s *BIOSDownloadScreen) Execute(config utils.Config, host romm.Host, platform romm.Platform) BIOSDownloadOutput {
	result, err := s.draw(BIOSDownloadInput{
		Config:   config,
		Host:     host,
		Platform: platform,
	})

	if err != nil {
		gaba.GetLogger().Error("BIOS download failed", "error", err)
		return BIOSDownloadOutput{Platform: platform}
	}

	return result.Value
}

func (s *BIOSDownloadScreen) draw(input BIOSDownloadInput) (ScreenResult[BIOSDownloadOutput], error) {
	logger := gaba.GetLogger()

	output := BIOSDownloadOutput{
		Platform: input.Platform,
	}

	// Fetch firmware list from RomM first
	client := utils.GetRommClient(input.Host, input.Config.ApiTimeout)
	firmwareList, err := client.GetFirmware(input.Platform.ID)
	if err != nil {
		logger.Error("Failed to fetch firmware from RomM", "error", err, "platform_id", input.Platform.ID)
		gaba.ConfirmationMessage(
			fmt.Sprintf("Failed to fetch BIOS files from RomM: %v", err),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return back(output), nil
	}

	if len(firmwareList) == 0 {
		logger.Info("No BIOS files available in RomM for platform", "platform", input.Platform.Name)
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "bios_no_files_required", Other: "This platform doesn't require any BIOS files."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return back(output), nil
	}

	logger.Debug("Fetched firmware from RomM", "count", len(firmwareList), "platform_id", input.Platform.ID)

	// Try to get BIOS metadata to enrich the firmware list (optional)
	biosFiles := utils.GetBIOSFilesForPlatform(input.Platform.Slug)

	// Build metadata lookup by filename for enrichment (case-insensitive)
	biosMetadataByFileName := make(map[string]constants.BIOSFile)
	biosMetadataByRelPath := make(map[string]constants.BIOSFile)
	for _, biosFile := range biosFiles {
		biosMetadataByFileName[strings.ToLower(biosFile.FileName)] = biosFile
		biosMetadataByRelPath[strings.ToLower(biosFile.RelativePath)] = biosFile
		baseName := filepath.Base(biosFile.RelativePath)
		if baseName != biosFile.RelativePath {
			biosMetadataByFileName[strings.ToLower(baseName)] = biosFile
		}
	}

	// Create a BIOSFile entry for each firmware, enriching with metadata if available
	type firmwareWithMetadata struct {
		firmware romm.Firmware
		metadata *constants.BIOSFile
	}

	var firmwareItems []firmwareWithMetadata
	for _, fw := range firmwareList {
		item := firmwareWithMetadata{firmware: fw}

		// Try to find matching metadata (case-insensitive)
		baseName := filepath.Base(fw.FilePath)
		if metadata, found := biosMetadataByFileName[strings.ToLower(fw.FileName)]; found {
			item.metadata = &metadata
		} else if metadata, found := biosMetadataByRelPath[strings.ToLower(fw.FilePath)]; found {
			item.metadata = &metadata
		} else if metadata, found := biosMetadataByFileName[strings.ToLower(baseName)]; found {
			item.metadata = &metadata
		}

		firmwareItems = append(firmwareItems, item)

		logger.Debug("RomM firmware entry",
			"filename", fw.FileName,
			"filepath", fw.FilePath,
			"size", fw.FileSizeBytes,
			"hasMetadata", item.metadata != nil)
	}

	var menuItems []gaba.MenuItem

	for _, item := range firmwareItems {
		fw := item.firmware
		var displayText string
		var shouldSelect bool

		if item.metadata != nil {
			// We have metadata - show enriched information
			status := utils.CheckBIOSFileStatus(*item.metadata, input.Platform.Slug)

			var statusText string
			switch status.Status {
			case utils.BIOSStatusValid:
				statusText = i18n.Localize(&goi18n.Message{ID: "bios_status_ready", Other: "Ready"}, nil)
			case utils.BIOSStatusInvalidHash:
				statusText = i18n.Localize(&goi18n.Message{ID: "bios_status_wrong_version", Other: "Wrong Version"}, nil)
			case utils.BIOSStatusNoHashToVerify:
				statusText = i18n.Localize(&goi18n.Message{ID: "bios_status_unverified", Other: "Installed (Unverified)"}, nil)
			case utils.BIOSStatusMissing:
				statusText = i18n.Localize(&goi18n.Message{ID: "bios_status_not_installed", Other: "Not Installed"}, nil)
			}

			optionalText := ""
			if item.metadata.Optional {
				optionalText = " (Optional)"
			}

			displayText = fmt.Sprintf("%s%s - %s", fw.FileName, optionalText, statusText)
			shouldSelect = status.Status == utils.BIOSStatusMissing || status.Status == utils.BIOSStatusInvalidHash
		} else {
			// No metadata - check if file exists and show basic status
			biosDir := utils.GetBIOSDirectory()

			// Try multiple potential file locations
			potentialPaths := []string{
				filepath.Join(biosDir, fw.FileName), // Root BIOS dir
				filepath.Join(biosDir, fw.FilePath), // Using firmware's relative path
			}

			fileExists := false
			for _, path := range potentialPaths {
				if _, err := os.Stat(path); err == nil {
					fileExists = true
					break
				}
			}

			var statusText string
			if fileExists {
				statusText = i18n.Localize(&goi18n.Message{ID: "bios_status_ready", Other: "Ready"}, nil)
				shouldSelect = false
			} else {
				statusText = i18n.Localize(&goi18n.Message{ID: "bios_status_not_installed", Other: "Not Installed"}, nil)
				shouldSelect = true
			}

			displayText = fmt.Sprintf("%s - %s", fw.FileName, statusText)
		}

		menuItems = append(menuItems, gaba.MenuItem{
			Text:     displayText,
			Selected: shouldSelect,
			Focused:  false,
			Metadata: item,
		})
	}

	options := gaba.DefaultListOptions(fmt.Sprintf("%s - BIOS", input.Platform.Name), menuItems)
	options.SmallTitle = true
	options.StartInMultiSelectMode = true
	options.FooterHelpItems = []gaba.FooterHelpItem{
		FooterBack(),
		{ButtonName: icons.Start, HelpText: i18n.Localize(&goi18n.Message{ID: "button_download", Other: "Download"}, nil), IsConfirmButton: true},
	}
	options.StatusBar = utils.StatusBar()

	sel, err := gaba.List(options)
	if err != nil {
		logger.Error("BIOS selection failed", "error", err)
		return back(output), err
	}

	if sel.Action != gaba.ListActionSelected || len(sel.Selected) == 0 {
		return back(output), nil
	}

	var selectedItems []firmwareWithMetadata
	for _, idx := range sel.Selected {
		item := sel.Items[idx].Metadata.(firmwareWithMetadata)
		selectedItems = append(selectedItems, item)
	}

	logger.Debug("Selected BIOS files for download", "count", len(selectedItems))

	// Build downloads from selected items
	var downloads []gaba.Download
	type downloadInfo struct {
		firmware romm.Firmware
		metadata *constants.BIOSFile
	}
	locationToInfoMap := make(map[string]downloadInfo)

	baseURL := input.Host.URL()
	for _, item := range selectedItems {
		downloadURL := baseURL + item.firmware.DownloadURL
		tempPath := filepath.Join(utils.TempDir(), fmt.Sprintf("bios_%s", item.firmware.FileName))

		downloads = append(downloads, gaba.Download{
			URL:         downloadURL,
			Location:    tempPath,
			DisplayName: item.firmware.FileName,
		})

		locationToInfoMap[tempPath] = downloadInfo{
			firmware: item.firmware,
			metadata: item.metadata,
		}

		logger.Debug("Added BIOS file to download queue",
			"file", item.firmware.FileName,
			"url", downloadURL,
			"size", item.firmware.FileSizeBytes)
	}

	headers := make(map[string]string)
	headers["Authorization"] = input.Host.BasicAuthHeader()

	res, err := gaba.DownloadManager(downloads, headers, gaba.DownloadManagerOptions{
		AutoContinue: true,
	})
	if err != nil {
		logger.Error("BIOS download failed", "error", err)
		return back(output), err
	}

	logger.Debug("Download results", "completed", len(res.Completed), "failed", len(res.Failed))

	successCount := 0
	warningCount := 0
	for _, download := range res.Completed {
		info := locationToInfoMap[download.Location]

		data, err := os.ReadFile(download.Location)
		if err != nil {
			logger.Error("Failed to read downloaded BIOS file", "file", info.firmware.FileName, "error", err)
			continue
		}

		// Verify MD5 if we have metadata
		if info.metadata != nil && info.metadata.MD5Hash != "" {
			isValid, actualHash := utils.VerifyBIOSFileMD5(data, info.metadata.MD5Hash)
			if !isValid {
				logger.Warn("MD5 hash mismatch for BIOS file",
					"file", info.metadata.FileName,
					"expected", info.metadata.MD5Hash,
					"actual", actualHash)
				warningCount++
			}
		}

		// Save the file
		if info.metadata != nil {
			// We have metadata - use SaveBIOSFile to handle subdirectories
			if err := utils.SaveBIOSFile(*info.metadata, input.Platform.Slug, data); err != nil {
				logger.Error("Failed to save BIOS file", "file", info.metadata.FileName, "error", err)
				continue
			}
		} else {
			// No metadata - use firmware FilePath from RomM (preserves subdirectories)
			biosDir := utils.GetBIOSDirectory()
			// Use FilePath if it contains directory info, otherwise just use FileName
			relativePath := info.firmware.FilePath
			if relativePath == "" || relativePath == info.firmware.FileName {
				relativePath = info.firmware.FileName
			}
			filePath := filepath.Join(biosDir, relativePath)

			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				logger.Error("Failed to create BIOS directory", "error", err)
				continue
			}

			if err := os.WriteFile(filePath, data, 0644); err != nil {
				logger.Error("Failed to save BIOS file", "file", info.firmware.FileName, "error", err)
				continue
			}
		}

		os.Remove(download.Location)
		successCount++
	}

	// Show completion message to user
	if successCount > 0 && warningCount == 0 {
		logger.Info("BIOS download complete", "success", successCount)
		gaba.ConfirmationMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "bios_download_complete", Other: "Successfully downloaded %d BIOS file(s)."}, nil), successCount),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	} else if successCount > 0 && warningCount > 0 {
		logger.Warn("BIOS download complete with warnings",
			"success", successCount,
			"warnings", warningCount)
		gaba.ConfirmationMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "bios_download_complete_with_warnings", Other: "Downloaded %d BIOS file(s) with %d hash warning(s). Files may not be the correct version."}, nil), successCount, warningCount),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	} else if len(res.Failed) > 0 {
		logger.Error("BIOS download failed", "failed", len(res.Failed))
		gaba.ConfirmationMessage(
			fmt.Sprintf(i18n.Localize(&goi18n.Message{ID: "bios_download_failed", Other: "Failed to download %d BIOS file(s)."}, nil), len(res.Failed)),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
	}

	return back(output), nil
}

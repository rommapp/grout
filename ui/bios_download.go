package ui

import (
	"fmt"
	"os"
	"path/filepath"

	"grout/bios"
	"grout/files"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	icons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type BIOSDownloadInput struct {
	Config   settings.Config
	Host     settings.Host
	Platform romm.Platform
}

type BIOSDownloadOutput struct {
	Platform romm.Platform
}

type BIOSDownloadScreen struct{}

func NewBIOSDownloadScreen() *BIOSDownloadScreen {
	return &BIOSDownloadScreen{}
}

func (s *BIOSDownloadScreen) Execute(config settings.Config, host settings.Host, platform romm.Platform) BIOSDownloadOutput {
	result, err := s.draw(BIOSDownloadInput{Config: config, Host: host, Platform: platform})
	if err != nil {
		gaba.GetLogger().Error("BIOS download failed", "error", err)
		return BIOSDownloadOutput{Platform: platform}
	}
	return result
}

func (s *BIOSDownloadScreen) draw(input BIOSDownloadInput) (BIOSDownloadOutput, error) {
	logger := gaba.GetLogger()
	output := BIOSDownloadOutput{Platform: input.Platform}

	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout.Duration())
	firmware, err := client.GetFirmware(input.Platform.ID)
	if err != nil {
		logger.Error("Failed to fetch firmware from RomM", "error", err, "platform_id", input.Platform.ID)
		s.tell(fmt.Sprintf("Failed to fetch BIOS files from RomM: %v", err))
		return output, nil
	}

	if len(firmware) == 0 {
		logger.Info("No BIOS files available in RomM for platform", "platform", input.Platform.Name)
		s.tell(localize("bios_no_files_required", "This platform doesn't require any BIOS files."))
		return output, nil
	}

	requirements := bios.Match(input.Platform.FSSlug, firmware)
	logger.Debug("Matched RomM firmware against known BIOS files",
		"platform", input.Platform.Name, "count", len(requirements))

	chosen, ok := s.choose(input.Platform, requirements)
	if !ok {
		return output, nil
	}

	installed, failed := s.fetch(input, chosen)

	switch {
	case installed > 0:
		logger.Info("BIOS download complete", "installed", installed, "failed", failed)
		s.tell(fmt.Sprintf(localize("bios_download_complete", "Successfully downloaded %d BIOS file(s)."), installed))
	case failed > 0:
		logger.Error("BIOS download failed", "failed", failed)
		s.tell(fmt.Sprintf(localize("bios_download_failed", "Failed to download %d BIOS file(s)."), failed))
	}

	return output, nil
}

// choose lists what the platform needs and lets the user pick. Anything
// missing starts selected, since that is what opening this screen asks for.
func (s *BIOSDownloadScreen) choose(platform romm.Platform, requirements []bios.Requirement) ([]bios.Requirement, bool) {
	items := make([]gaba.MenuItem, 0, len(requirements))
	for _, requirement := range requirements {
		missing := !requirement.Installed(platform.FSSlug)
		items = append(items, gaba.MenuItem{
			Text:     biosLabel(requirement, missing),
			Selected: missing,
			Metadata: requirement,
		})
	}

	options := gaba.DefaultListOptions(fmt.Sprintf("%s - BIOS", platform.Name), items)
	options.UseSmallTitle = true
	options.InitialMultiSelectMode = true
	options.StatusBar = StatusBar()
	options.FooterHelpItems = []gaba.FooterHelpItem{
		FooterBack(),
		{ButtonName: icons.Start, HelpText: localize("button_download", "Download"), IsConfirmButton: true},
	}

	result, err := gaba.List(options)
	if err != nil || result.Action != gaba.ListActionSelected || len(result.Selected) == 0 {
		return nil, false
	}

	chosen := make([]bios.Requirement, 0, len(result.Selected))
	for _, index := range result.Selected {
		chosen = append(chosen, result.Items[index].Metadata.(bios.Requirement))
	}
	return chosen, true
}

func biosLabel(requirement bios.Requirement, missing bool) string {
	status := localize("bios_status_ready", "Ready")
	if missing {
		status = localize("bios_status_not_installed", "Missing")
	}

	optional := ""
	// Only say a file is optional when the tables actually say so. An
	// unmatched entry is not a claim that the emulator runs without it.
	if requirement.Known && requirement.File.Optional {
		optional = " " + localize("bios_optional", "(Optional)")
	}

	return fmt.Sprintf("%s%s - %s", requirement.Firmware.FileName, optional, status)
}

// fetch downloads the chosen files and installs each one everywhere the
// firmware expects it.
//
// They land in a temp directory first because one BIOS file can belong in
// several places, and the download widget writes to a single location.
func (s *BIOSDownloadScreen) fetch(input BIOSDownloadInput, chosen []bios.Requirement) (installed, failed int) {
	logger := gaba.GetLogger()

	downloads := make([]gaba.Download, 0, len(chosen))
	staged := make(map[string]bios.Requirement, len(chosen))
	for _, requirement := range chosen {
		location := filepath.Join(files.TempDir(), "bios_"+requirement.Firmware.FileName)
		downloads = append(downloads, gaba.Download{
			URL:         input.Host.URL() + requirement.Firmware.DownloadURL,
			Location:    location,
			DisplayName: requirement.Firmware.FileName,
		})
		staged[location] = requirement
	}

	// Nothing here should outlive the run: an install copies the file where it
	// belongs, and a failure leaves a partial one worth removing.
	defer func() {
		for location := range staged {
			files.DeleteFile(location)
		}
	}()

	result, err := gaba.DownloadManager(downloads, map[string]string{
		"Authorization": input.Host.AuthHeader(),
	}, gaba.DownloadManagerOptions{
		AutoContinueOnComplete: true,
		SkipSSLVerification:    input.Host.InsecureSkipVerify,
	})
	if err != nil {
		logger.Error("BIOS download failed", "error", err)
		return 0, len(downloads)
	}

	logger.Debug("Download results", "completed", len(result.Completed), "failed", len(result.Failed))
	failed = len(result.Failed)

	for _, download := range result.Completed {
		requirement := staged[download.Location]

		data, err := os.ReadFile(download.Location)
		if err != nil {
			logger.Error("Failed to read downloaded BIOS file", "file", requirement.Firmware.FileName, "error", err)
			failed++
			continue
		}

		if err := requirement.Save(input.Platform.FSSlug, data); err != nil {
			logger.Error("Failed to save BIOS file", "file", requirement.Firmware.FileName, "error", err)
			failed++
			continue
		}

		installed++
	}

	return installed, failed
}

func (s *BIOSDownloadScreen) tell(message string) {
	gaba.ConfirmationMessage(message, ContinueFooter(), gaba.MessageOptions{})
}

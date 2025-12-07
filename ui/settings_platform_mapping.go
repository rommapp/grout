package ui

import (
	"errors"
	"fmt"
	"grout/models"
	"grout/utils"
	"os"
	"slices"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/brandonkowalski/go-romm"
)

// PlatformMappingInput contains data needed to render the platform mapping screen
type PlatformMappingInput struct {
	Host           models.Host
	ApiTimeout     time.Duration
	CFW            models.CFW
	RomDirectory   string // Base ROM directory path
	AutoSelect     bool   // Auto-select "Create" option when no match found
	HideBackButton bool   // Hide the back/cancel button
}

// PlatformMappingOutput contains the result of the platform mapping screen
type PlatformMappingOutput struct {
	Mappings map[string]models.DirectoryMapping
}

// PlatformMappingScreen displays platform to directory mapping configuration
type PlatformMappingScreen struct{}

func NewPlatformMappingScreen() *PlatformMappingScreen {
	return &PlatformMappingScreen{}
}

func (s *PlatformMappingScreen) Draw(input PlatformMappingInput) (gaba.ScreenResult[PlatformMappingOutput], error) {
	logger := gaba.GetLogger()
	output := PlatformMappingOutput{Mappings: make(map[string]models.DirectoryMapping)}

	// Fetch RomM platforms
	rommPlatforms, err := s.fetchPlatforms(input)
	if err != nil {
		logger.Error("Error fetching RomM Platforms", "error", err)
		return gaba.WithCode(output, gaba.ExitCodeError), err
	}

	// Get local ROM directories
	romDirectories, err := s.getRomDirectories(input.RomDirectory)
	if err != nil {
		logger.Error("Error fetching ROM directories", "error", err)
		return gaba.WithCode(output, gaba.ExitCodeBack), err
	}

	// Build mapping options
	mappingOptions := s.buildMappingOptions(rommPlatforms, romDirectories, input)

	// Configure footer
	footerItems := []gaba.FooterHelpItem{
		{ButtonName: "←→", HelpText: "Cycle"},
		{ButtonName: "Start", HelpText: "Save"},
	}
	if !input.HideBackButton {
		footerItems = slices.Insert(footerItems, 0, gaba.FooterHelpItem{ButtonName: "B", HelpText: "Cancel"})
	}

	// Show options list
	result, err := gaba.OptionsList(
		"Rom Directory Mapping",
		gaba.OptionListSettings{
			FooterHelpItems:   footerItems,
			DisableBackButton: input.HideBackButton,
		},
		mappingOptions,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return gaba.Back(PlatformMappingOutput{}), nil
		}
		return gaba.WithCode(PlatformMappingOutput{}, gaba.ExitCodeError), err
	}

	// Build mappings from result
	output.Mappings = s.buildMappingsFromResult(result.Items)

	return gaba.Success(output), nil
}

func (s *PlatformMappingScreen) fetchPlatforms(input PlatformMappingInput) ([]romm.Platform, error) {
	client := romm.NewClient(
		input.Host.URL(),
		romm.WithBasicAuth(input.Host.Username, input.Host.Password),
		romm.WithTimeout(input.ApiTimeout),
	)
	return client.GetPlatforms()
}

func (s *PlatformMappingScreen) getRomDirectories(romDir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(romDir)
	if err != nil {
		gaba.ConfirmationMessage("ROM Directory Could Not Be Found!", []gaba.FooterHelpItem{
			{ButtonName: "B", HelpText: "Quit"},
		}, gaba.MessageOptions{})
		return nil, fmt.Errorf("failed to read ROM directory: %w", err)
	}

	var dirs []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry)
		}
	}

	return dirs, nil
}

func (s *PlatformMappingScreen) buildMappingOptions(
	platforms []romm.Platform,
	romDirectories []os.DirEntry,
	input PlatformMappingInput,
) []gaba.ItemWithOptions {
	options := make([]gaba.ItemWithOptions, 0, len(platforms))

	for _, platform := range platforms {
		platformOptions, selectedIndex := s.buildPlatformOptions(platform, romDirectories, input)

		options = append(options, gaba.ItemWithOptions{
			Item: gaba.MenuItem{
				Text:     platform.Name,
				Metadata: platform.Slug,
			},
			Options:        platformOptions,
			SelectedOption: selectedIndex,
		})
	}

	return options
}

func (s *PlatformMappingScreen) buildPlatformOptions(
	platform romm.Platform,
	romDirectories []os.DirEntry,
	input PlatformMappingInput,
) ([]gaba.Option, int) {
	// Start with "Skip" option
	options := []gaba.Option{{DisplayName: "Skip", Value: ""}}
	selectedIndex := 0
	canCreate := false

	// Check if we can auto-match or need to create
	matchIndex := s.findMatchingDirectory(platform, romDirectories, input.CFW)

	if matchIndex == -1 {
		// No match found - add "Create" option if possible
		displayName := s.getCreateDisplayName(platform.Slug, input.CFW)
		if displayName != "" {
			options = append(options, gaba.Option{
				DisplayName: fmt.Sprintf("Create '%s'", displayName),
				Value:       utils.RomMSlugToCFW(platform.Slug),
			})
			canCreate = true
		}
	}

	// Add all existing ROM directories as options
	for _, romDir := range romDirectories {
		dirName := romDir.Name()
		displayName := dirName
		if input.CFW == models.NEXTUI {
			displayName = utils.ParseTag(dirName)
		}

		options = append(options, gaba.Option{
			DisplayName: fmt.Sprintf("/%s", displayName),
			Value:       dirName,
		})

		// Check if this directory matches the platform
		if s.directoryMatchesPlatform(platform, romDir.Name(), input.CFW) {
			selectedIndex = len(options) - 1
		}
	}

	// Auto-select "Create" if appropriate
	if selectedIndex == 0 && len(options) > 1 && (len(romDirectories) == 0 || (canCreate && input.AutoSelect)) {
		selectedIndex = 1
	}

	return options, selectedIndex
}

func (s *PlatformMappingScreen) findMatchingDirectory(
	platform romm.Platform,
	romDirectories []os.DirEntry,
	cfw models.CFW,
) int {
	for i, entry := range romDirectories {
		if s.directoryMatchesPlatform(platform, entry.Name(), cfw) {
			return i
		}
	}
	return -1
}

func (s *PlatformMappingScreen) directoryMatchesPlatform(
	platform romm.Platform,
	dirName string,
	cfw models.CFW,
) bool {
	cfwSlug := utils.RomMSlugToCFW(platform.Slug)
	romFolderBase := utils.RomFolderBase(dirName)

	switch cfw {
	case models.NEXTUI:
		return utils.ParseTag(cfwSlug) == romFolderBase
	default:
		return cfwSlug == romFolderBase
	}
}

func (s *PlatformMappingScreen) getCreateDisplayName(slug string, cfw models.CFW) string {
	displayName := utils.RomMSlugToCFW(slug)
	if cfw == models.NEXTUI {
		displayName = utils.ParseTag(displayName)
	}
	return displayName
}

func (s *PlatformMappingScreen) buildMappingsFromResult(items []gaba.ItemWithOptions) map[string]models.DirectoryMapping {
	mappings := make(map[string]models.DirectoryMapping)

	for _, item := range items {
		rommSlug := item.Item.Metadata.(string)
		relativePath := item.Options[item.SelectedOption].Value.(string)

		// Skip empty mappings
		if relativePath == "" {
			continue
		}

		mappings[rommSlug] = models.DirectoryMapping{
			RomMSlug:     rommSlug,
			RelativePath: relativePath,
		}
	}

	return mappings
}

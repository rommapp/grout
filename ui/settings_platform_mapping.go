package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/cfw"
	"grout/internal"
	"grout/internal/fileutil"
	"grout/internal/stringutil"
	"os"
	"path/filepath"
	"slices"
	"time"

	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type PlatformMappingInput struct {
	Host             romm.Host
	ApiTimeout       time.Duration
	CFW              cfw.CFW
	RomDirectory     string
	AutoSelect       bool
	HideBackButton   bool
	ExistingMappings map[string]internal.DirectoryMapping // For return visits, use existing config
	PlatformsBinding map[string]string                    // fs_slug -> bound slug for CFW lookups
}

type PlatformMappingOutput struct {
	Mappings map[string]internal.DirectoryMapping
}

type PlatformMappingScreen struct{}

func NewPlatformMappingScreen() *PlatformMappingScreen {
	return &PlatformMappingScreen{}
}

func (s *PlatformMappingScreen) Draw(input PlatformMappingInput) (ScreenResult[PlatformMappingOutput], error) {
	logger := gaba.GetLogger()
	output := PlatformMappingOutput{Mappings: make(map[string]internal.DirectoryMapping)}

	rommPlatforms, err := s.fetchPlatforms(input)
	if err != nil {
		logger.Error("Error fetching RomM Platforms", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	romDirectories, err := s.getRomDirectories(input.RomDirectory)
	if err != nil {
		logger.Error("Error fetching ROM directories", "error", err)
		return withCode(output, gaba.ExitCodeBack), err
	}

	mappingOptions := s.buildMappingOptions(rommPlatforms, romDirectories, input)

	footerItems := []gaba.FooterHelpItem{
		FooterCycle(),
		FooterSave(),
	}
	if !input.HideBackButton {
		footerItems = slices.Insert(footerItems, 0, FooterCancel())
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "platform_mapping_title", Other: "Rom Directory Mapping"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems:   footerItems,
			DisableBackButton: input.HideBackButton,
			StatusBar:         StatusBar(),
		},
		mappingOptions,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return back(PlatformMappingOutput{}), nil
		}
		return withCode(PlatformMappingOutput{}, gaba.ExitCodeError), err
	}

	output.Mappings = s.buildMappingsFromResult(result.Items)

	if err := s.createDirectories(output.Mappings, input.RomDirectory, romDirectories); err != nil {
		logger.Error("Error creating directories", "error", err)
		return withCode(output, gaba.ExitCodeError), err
	}

	return success(output), nil
}

func (s *PlatformMappingScreen) fetchPlatforms(input PlatformMappingInput) ([]romm.Platform, error) {
	if cm := cache.GetCacheManager(); cm != nil {
		if platforms, err := cm.GetPlatforms(); err == nil && len(platforms) > 0 {
			romm.DisambiguatePlatformNames(platforms)
			return platforms, nil
		}
	}

	client := romm.NewClientFromHost(input.Host, input.ApiTimeout)
	platforms, err := client.GetPlatforms()
	if err != nil {
		return nil, err
	}
	romm.DisambiguatePlatformNames(platforms)
	return platforms, nil
}

func (s *PlatformMappingScreen) getRomDirectories(romDir string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(romDir)
	if err != nil {
		gaba.ConfirmationMessage(i18n.Localize(&goi18n.Message{ID: "platform_mapping_directory_not_found", Other: "ROM Directory Could Not Be Found!"}, nil), []gaba.FooterHelpItem{
			FooterQuit(),
		}, gaba.MessageOptions{})
		return nil, fmt.Errorf("failed to read ROM directory: %w", err)
	}

	return fileutil.FilterHiddenDirectories(entries), nil
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
				Metadata: platform.FSSlug,
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
	options := []gaba.Option{{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_skip", Other: "Skip"}, nil), Value: ""}}
	selectedIndex := 0

	cfwDirectories := s.getCFWDirectoriesForPlatform(platform.FSSlug, input.CFW, input.PlatformsBinding)

	// Check if this is a return visit with existing mappings
	hasExistingMappings := len(input.ExistingMappings) > 0
	existingMapping, platformHasMapping := input.ExistingMappings[platform.FSSlug]

	createOptionAdded := false
	for _, cfwDir := range cfwDirectories {
		dirExists := false
		for _, romDir := range romDirectories {
			if s.directoriesMatch(cfwDir, romDir.Name(), input.CFW) {
				dirExists = true
				break
			}
		}

		if !dirExists {
			displayName := cfwDir
			if input.CFW == cfw.NextUI {
				displayName = stringutil.ParseTag(cfwDir)
			}
			options = append(options, gaba.Option{
				DisplayName: i18n.Localize(&goi18n.Message{ID: "platform_mapping_create", Other: "Create '{{.Name}}'"}, map[string]interface{}{"Name": displayName}),
				Value:       cfwDir,
			})
			createOptionAdded = true

			// For return visits, select if this matches the existing mapping
			if hasExistingMappings && platformHasMapping && cfwDir == existingMapping.RelativePath {
				selectedIndex = len(options) - 1
			}
		}
	}

	for _, romDir := range romDirectories {
		dirName := romDir.Name()

		if s.isValidDirectoryForPlatform(dirName, input.CFW, cfwDirectories) {
			displayName := dirName
			if input.CFW == cfw.NextUI {
				displayName = stringutil.ParseTag(dirName)
			}

			options = append(options, gaba.Option{
				DisplayName: i18n.Localize(&goi18n.Message{ID: "platform_mapping_path_prefix", Other: "/{{.Name}}"}, map[string]interface{}{"Name": displayName}),
				Value:       dirName,
			})

			if hasExistingMappings {
				// For return visits, only select if this platform has a mapping and it matches
				if platformHasMapping && dirName == existingMapping.RelativePath {
					selectedIndex = len(options) - 1
				}
			} else {
				// First time: auto-detect based on directory name matching platform
				if s.directoryMatchesPlatform(platform, romDir.Name(), input.CFW) {
					selectedIndex = len(options) - 1
				}
			}
		}
	}

	// Only auto-select create option on first run (not return visits)
	if !hasExistingMappings && selectedIndex == 0 && createOptionAdded && input.AutoSelect {
		selectedIndex = 1
	}

	return options, selectedIndex
}

func (s *PlatformMappingScreen) directoryMatchesPlatform(
	platform romm.Platform,
	dirName string,
	c cfw.CFW,
) bool {
	cfwFSSlug := cfw.RomMFSSlugToCFW(platform.FSSlug)
	romFolderBase := cfw.RomFolderBase(dirName, stringutil.ParseTag)

	switch c {
	case cfw.NextUI:
		return stringutil.ParseTag(cfwFSSlug) == romFolderBase
	default:
		return cfwFSSlug == romFolderBase
	}
}

func (s *PlatformMappingScreen) getCFWDirectoriesForPlatform(fsSlug string, c cfw.CFW, platformsBinding map[string]string) []string {
	// Resolve fsSlug through platform binding if available
	effectiveSlug := fsSlug
	if platformsBinding != nil {
		if bound, ok := platformsBinding[fsSlug]; ok {
			gaba.GetLogger().Debug("Using platform binding for CFW lookup",
				"fsSlug", fsSlug, "boundTo", bound)
			effectiveSlug = bound
		}
	}

	platformMap := cfw.GetPlatformMap(c)
	if platformMap != nil {
		if dirs, ok := platformMap[effectiveSlug]; ok && len(dirs) > 0 {
			return dirs
		}
	}
	// Fall back to effective slug if no CFW-specific mapping exists
	return []string{effectiveSlug}
}

func (s *PlatformMappingScreen) directoriesMatch(dir1, dir2 string, c cfw.CFW) bool {
	if c == cfw.NextUI {
		return stringutil.ParseTag(dir1) == stringutil.ParseTag(dir2)
	}
	return dir1 == dir2
}

func (s *PlatformMappingScreen) isValidDirectoryForPlatform(dirName string, c cfw.CFW, cfwDirectories []string) bool {
	for _, cfwDir := range cfwDirectories {
		if s.directoriesMatch(cfwDir, dirName, c) {
			return true
		}
	}
	return false
}

func (s *PlatformMappingScreen) buildMappingsFromResult(items []gaba.ItemWithOptions) map[string]internal.DirectoryMapping {
	mappings := make(map[string]internal.DirectoryMapping)

	for _, item := range items {
		rommSlug := item.Item.Metadata.(string)
		relativePath := item.Options[item.SelectedOption].Value.(string)

		if relativePath == "" {
			continue
		}

		mappings[rommSlug] = internal.DirectoryMapping{
			RomMSlug:     rommSlug,
			RelativePath: relativePath,
		}
	}

	return mappings
}

func (s *PlatformMappingScreen) createDirectories(
	mappings map[string]internal.DirectoryMapping,
	romDirectory string,
	existingDirs []os.DirEntry,
) error {
	logger := gaba.GetLogger()

	existingDirMap := make(map[string]bool)
	for _, dir := range existingDirs {
		existingDirMap[dir.Name()] = true
	}

	for _, mapping := range mappings {
		if existingDirMap[mapping.RelativePath] {
			continue
		}

		fullPath := filepath.Join(romDirectory, mapping.RelativePath)
		logger.Debug("Creating new ROM directory", "path", fullPath)

		if err := os.MkdirAll(fullPath, 0755); err != nil {
			logger.Error("Failed to create directory", "path", fullPath, "error", err)
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}

		logger.Info("Created ROM directory", "path", fullPath)
	}

	return nil
}

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
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
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
	Action   PlatformMappingAction
	Mappings map[string]internal.DirectoryMapping
}

type PlatformMappingScreen struct{}

func NewPlatformMappingScreen() *PlatformMappingScreen {
	return &PlatformMappingScreen{}
}

// distinctPlatformValues returns the sorted, deduplicated, non-empty values produced
// by get across platforms. An empty result means the corresponding metadata filter has
// no options and should be hidden rather than shown as an "All"-only picker.
func distinctPlatformValues(platforms []romm.Platform, get func(romm.Platform) string) []string {
	set := make(map[string]bool)
	for _, p := range platforms {
		if v := get(p); v != "" {
			set[v] = true
		}
	}
	values := make([]string, 0, len(set))
	for v := range set {
		values = append(values, v)
	}
	slices.Sort(values)
	return values
}

func (s *PlatformMappingScreen) Draw(input PlatformMappingInput) (PlatformMappingOutput, error) {
	logger := gaba.GetLogger()
	output := PlatformMappingOutput{Action: PlatformMappingActionBack, Mappings: make(map[string]internal.DirectoryMapping)}

	rommPlatforms, err := s.fetchPlatforms(input)
	if err != nil {
		logger.Error("Error fetching RomM Platforms", "error", err)
		return output, err
	}

	romDirectories, err := s.getRomDirectories(input.RomDirectory)
	if err != nil {
		logger.Error("Error fetching ROM directories", "error", err)
		return output, err
	}

	showGamesOnly := false
	mappingStatus := "all"
	categoryFilter := "all"
	familyFilter := "all"
	generationFilter := 0
	selectedIndex := 0
	visibleStartIndex := 0

	// We copy the existing mappings so we can update/accumulate them as the user interacts.
	currentMappings := make(map[string]internal.DirectoryMapping)
	for k, v := range input.ExistingMappings {
		currentMappings[k] = v
	}

	// If this is the first visit (no existing mappings), we can build options once to run the auto-detection,
	// and capture the auto-detected selections into currentMappings.
	if len(input.ExistingMappings) == 0 {
		initialOptions := s.buildMappingOptions(rommPlatforms, romDirectories, input)
		for _, item := range initialOptions {
			rommSlug := item.Item.Metadata.(string)
			relativePath := item.Options[item.SelectedOption].Value.(string)
			if relativePath != "" {
				currentMappings[rommSlug] = internal.DirectoryMapping{
					RomMSlug:     rommSlug,
					RelativePath: relativePath,
				}
			}
		}
	}

	for {
		var filteredPlatforms []romm.Platform
		for _, p := range rommPlatforms {
			if showGamesOnly && p.ROMCount == 0 {
				continue
			}
			if generationFilter != 0 && p.Generation != generationFilter {
				continue
			}
			if categoryFilter != "all" && p.Category != categoryFilter {
				continue
			}
			if familyFilter != "all" && p.Family != familyFilter {
				continue
			}
			if mappingStatus == "mapped" {
				if mapping, ok := currentMappings[p.FSSlug]; !ok || mapping.RelativePath == "" {
					continue
				}
			} else if mappingStatus == "unmapped" {
				if mapping, ok := currentMappings[p.FSSlug]; ok && mapping.RelativePath != "" {
					continue
				}
			}
			filteredPlatforms = append(filteredPlatforms, p)
		}

		// Update input with currentMappings to preserve state
		input.ExistingMappings = currentMappings

		mappingOptions := s.buildMappingOptions(filteredPlatforms, romDirectories, input)

		if len(mappingOptions) == 0 {
			mappingOptions = []gaba.ItemWithOptions{
				{
					Item: gaba.MenuItem{
						Text:     i18n.Localize(&goi18n.Message{ID: "platform_mapping_no_results", Other: "No matching platforms found. Press Y to filter."}, nil),
						Metadata: "dummy_no_results",
					},
					Options: []gaba.Option{
						{DisplayName: "", Value: ""},
					},
					SelectedOption: 0,
				},
			}
		}

		footerItems := []gaba.FooterHelpItem{
			FooterCycle(),
			FooterSelect(),
		}
		if !input.HideBackButton {
			footerItems = slices.Insert(footerItems, 0, FooterCancel())
		}
		footerItems = append(footerItems, gaba.FooterHelpItem{
			ButtonName: "Y",
			HelpText:   i18n.Localize(&goi18n.Message{ID: "button_filters", Other: "Filters"}, nil),
		})
		footerItems = append(footerItems, FooterSave())

		result, err := gaba.OptionsList(
			i18n.Localize(&goi18n.Message{ID: "platform_mapping_title", Other: "Rom Directory Mapping"}, nil),
			gaba.OptionListSettings{
				InitialSelectedIndex:  selectedIndex,
				VisibleStartIndex:     visibleStartIndex,
				FooterHelpItems:       footerItems,
				DisableBackButton:     input.HideBackButton,
				StatusBar:             StatusBar(),
				ListPickerButton:      constants.VirtualButtonA,
				SecondaryActionButton: constants.VirtualButtonY,
			},
			mappingOptions,
		)

		if err != nil {
			if errors.Is(err, gaba.ErrCancelled) {
				return PlatformMappingOutput{Action: PlatformMappingActionBack}, nil
			}
			return output, err
		}

		// Update current mappings from the current screen state so we don't lose changes.
		for _, item := range result.Items {
			rommSlug := item.Item.Metadata.(string)
			relativePath := item.Options[item.SelectedOption].Value.(string)
			currentMappings[rommSlug] = internal.DirectoryMapping{
				RomMSlug:     rommSlug,
				RelativePath: relativePath,
			}
		}

		if result.Action == gaba.ListActionSecondaryTriggered {
			// Find the slug of the item we were on so we can refocus on it
			var lastSelectedSlug string
			if result.Selected >= 0 && result.Selected < len(mappingOptions) {
				lastSelectedSlug = mappingOptions[result.Selected].Item.Metadata.(string)
			}

			showGamesOnlySub := showGamesOnly
			mappingStatusSub := mappingStatus
			categoryFilterSub := categoryFilter
			familyFilterSub := familyFilter
			generationFilterSub := generationFilter

			for {
				// Build dynamic list of generations present in rommPlatforms
				generationsSet := make(map[int]bool)
				for _, p := range rommPlatforms {
					if p.Generation > 0 {
						generationsSet[p.Generation] = true
					}
				}
				var uniqueGenerations []int
				for gen := range generationsSet {
					uniqueGenerations = append(uniqueGenerations, gen)
				}
				slices.Sort(uniqueGenerations)

				generationOptions := []gaba.Option{
					{DisplayName: i18n.Localize(&goi18n.Message{ID: "filter_all", Other: "All"}, nil), Value: 0},
				}
				generationSelectedIndex := 0
				for idx, gen := range uniqueGenerations {
					displayName := fmt.Sprintf("Generation %d", gen)
					generationOptions = append(generationOptions, gaba.Option{
						DisplayName: displayName,
						Value:       gen,
					})
					if gen == generationFilterSub {
						generationSelectedIndex = idx + 1
					}
				}

				// Build dynamic list of categories present in rommPlatforms
				uniqueCategories := distinctPlatformValues(rommPlatforms, func(p romm.Platform) string { return p.Category })

				categoryOptions := []gaba.Option{
					{DisplayName: i18n.Localize(&goi18n.Message{ID: "filter_all", Other: "All"}, nil), Value: "all"},
				}
				categorySelectedIndex := 0
				for idx, c := range uniqueCategories {
					categoryOptions = append(categoryOptions, gaba.Option{
						DisplayName: c,
						Value:       c,
					})
					if c == categoryFilterSub {
						categorySelectedIndex = idx + 1
					}
				}

				// Build dynamic list of families present in rommPlatforms
				uniqueFamilies := distinctPlatformValues(rommPlatforms, func(p romm.Platform) string { return p.Family })

				familyOptions := []gaba.Option{
					{DisplayName: i18n.Localize(&goi18n.Message{ID: "filter_all", Other: "All"}, nil), Value: "all"},
				}
				familySelectedIndex := 0
				for idx, f := range uniqueFamilies {
					familyOptions = append(familyOptions, gaba.Option{
						DisplayName: f,
						Value:       f,
					})
					if f == familyFilterSub {
						familySelectedIndex = idx + 1
					}
				}

				filterItems := []gaba.ItemWithOptions{
					{
						Item: gaba.MenuItem{
							Text: i18n.Localize(&goi18n.Message{ID: "settings_mapping_status", Other: "Mapping Status"}, nil),
						},
						Options: []gaba.Option{
							{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_mapping_status_all", Other: "All"}, nil), Value: "all"},
							{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_mapping_status_mapped", Other: "Mapped"}, nil), Value: "mapped"},
							{DisplayName: i18n.Localize(&goi18n.Message{ID: "settings_mapping_status_unmapped", Other: "Unmapped"}, nil), Value: "unmapped"},
						},
						SelectedOption: mappingStatusToIndex(mappingStatusSub),
					},
					{
						Item: gaba.MenuItem{
							Text: i18n.Localize(&goi18n.Message{ID: "settings_only_show_platforms_with_games", Other: "Only Platforms with Games"}, nil),
						},
						Options: []gaba.Option{
							{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_false", Other: "False"}, nil), Value: false},
							{DisplayName: i18n.Localize(&goi18n.Message{ID: "common_true", Other: "True"}, nil), Value: true},
						},
						SelectedOption: boolToIndex(showGamesOnlySub),
					},
				}

				// Only show a metadata filter when RomM actually populated values for it —
				// otherwise it's a useless "All"-only picker (#247). Category/Family are
				// frequently empty (they require IGDB platform metadata); Generation usually
				// has values but is gated the same way for consistency.
				if len(uniqueCategories) > 0 {
					filterItems = append(filterItems, gaba.ItemWithOptions{
						Item: gaba.MenuItem{
							Text: i18n.Localize(&goi18n.Message{ID: "settings_category", Other: "Category"}, nil),
						},
						Options:        categoryOptions,
						SelectedOption: categorySelectedIndex,
					})
				}
				if len(uniqueFamilies) > 0 {
					filterItems = append(filterItems, gaba.ItemWithOptions{
						Item: gaba.MenuItem{
							Text: i18n.Localize(&goi18n.Message{ID: "settings_family", Other: "Family"}, nil),
						},
						Options:        familyOptions,
						SelectedOption: familySelectedIndex,
					})
				}
				if len(uniqueGenerations) > 0 {
					filterItems = append(filterItems, gaba.ItemWithOptions{
						Item: gaba.MenuItem{
							Text: i18n.Localize(&goi18n.Message{ID: "settings_generation", Other: "Generation"}, nil),
						},
						Options:        generationOptions,
						SelectedOption: generationSelectedIndex,
					})
				}

				filterResult, err := gaba.OptionsList(
					i18n.Localize(&goi18n.Message{ID: "game_filters_title", Other: "Filters"}, nil),
					gaba.OptionListSettings{
						FooterHelpItems: []gaba.FooterHelpItem{
							FooterCancel(),
							FooterCycle(),
							{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_reset", Other: "Reset"}, nil)},
							FooterSave(),
						},
						DisableBackButton: false,
						StatusBar:         StatusBar(),
						ListPickerButton:  constants.VirtualButtonA,
						ActionButton:      constants.VirtualButtonX,
						UseSmallTitle:     true,
					},
					filterItems,
				)

				if err != nil {
					// Cancel (B button) -> exit sub-menu loop without saving
					break
				}

				// If X button (Reset) was pressed, reset submenu variables and reload
				if filterResult.Action == gaba.ListActionTriggered {
					showGamesOnlySub = false
					mappingStatusSub = "all"
					categoryFilterSub = "all"
					familyFilterSub = "all"
					generationFilterSub = 0
					continue
				}

				// Update values from submenu and exit
				for _, item := range filterResult.Items {
					switch item.Item.Text {
					case i18n.Localize(&goi18n.Message{ID: "settings_only_show_platforms_with_games", Other: "Only Platforms with Games"}, nil):
						if val, ok := item.Options[item.SelectedOption].Value.(bool); ok {
							showGamesOnly = val
						}
					case i18n.Localize(&goi18n.Message{ID: "settings_mapping_status", Other: "Mapping Status"}, nil):
						if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
							mappingStatus = val
						}
					case i18n.Localize(&goi18n.Message{ID: "settings_category", Other: "Category"}, nil):
						if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
							categoryFilter = val
						}
					case i18n.Localize(&goi18n.Message{ID: "settings_family", Other: "Family"}, nil):
						if val, ok := item.Options[item.SelectedOption].Value.(string); ok {
							familyFilter = val
						}
					case i18n.Localize(&goi18n.Message{ID: "settings_generation", Other: "Generation"}, nil):
						if val, ok := item.Options[item.SelectedOption].Value.(int); ok {
							generationFilter = val
						}
					}
				}
				break
			}

			// Re-filter platforms and build options to find the new index of lastSelectedSlug
			var nextFilteredPlatforms []romm.Platform
			for _, p := range rommPlatforms {
				if showGamesOnly && p.ROMCount == 0 {
					continue
				}
				if generationFilter != 0 && p.Generation != generationFilter {
					continue
				}
				if categoryFilter != "all" && p.Category != categoryFilter {
					continue
				}
				if familyFilter != "all" && p.Family != familyFilter {
					continue
				}
				if mappingStatus == "mapped" {
					if mapping, ok := currentMappings[p.FSSlug]; !ok || mapping.RelativePath == "" {
						continue
					}
				} else if mappingStatus == "unmapped" {
					if mapping, ok := currentMappings[p.FSSlug]; ok && mapping.RelativePath != "" {
						continue
					}
				}
				nextFilteredPlatforms = append(nextFilteredPlatforms, p)
			}
			nextOptions := s.buildMappingOptions(nextFilteredPlatforms, romDirectories, input)

			selectedIndex = 0
			if lastSelectedSlug != "" {
				for idx, opt := range nextOptions {
					if opt.Item.Metadata.(string) == lastSelectedSlug {
						selectedIndex = idx
						break
					}
				}
			}
			// Reset visible start index or estimate it
			visibleStartIndex = max(0, selectedIndex-(result.Selected-result.VisibleStartIndex))
			continue
		}

		// Compile final mappings from currentMappings
		finalMappings := make(map[string]internal.DirectoryMapping)
		for slug, mapping := range currentMappings {
			if mapping.RelativePath != "" && slug != "dummy_no_results" {
				finalMappings[slug] = mapping
			}
		}
		output.Mappings = finalMappings
		break
	}

	if err := s.createDirectories(output.Mappings, input.RomDirectory, romDirectories); err != nil {
		logger.Error("Error creating directories", "error", err)
		return output, err
	}

	output.Action = PlatformMappingActionSaved
	return output, nil
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
		gaba.GetLogger().Error("failed to read ROM directory", "error", err)
		os.Exit(1)
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
			if input.CFW == cfw.NextUI || input.CFW == cfw.MinUI {
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
			if input.CFW == cfw.NextUI || input.CFW == cfw.MinUI {
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

	// Check if existing mapping is a custom value (not matched by any predefined option)
	isCustomValue := hasExistingMappings && platformHasMapping && existingMapping.RelativePath != "" && selectedIndex == 0
	customDisplayName := i18n.Localize(&goi18n.Message{ID: "platform_mapping_custom", Other: "Custom..."}, nil)
	customValue := ""

	if isCustomValue {
		customDisplayName = existingMapping.RelativePath
		customValue = existingMapping.RelativePath
	}

	// Add custom input option at the end
	options = append(options, gaba.Option{
		DisplayName:    customDisplayName,
		Value:          customValue,
		Type:           gaba.OptionTypeKeyboard,
		KeyboardPrompt: customValue, // Prepopulate keyboard with existing value
	})

	// Select custom option if it has a value
	if isCustomValue {
		selectedIndex = len(options) - 1
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
	case cfw.NextUI, cfw.MinUI:
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
	if c == cfw.NextUI || c == cfw.MinUI {
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

func mappingStatusToIndex(status string) int {
	switch status {
	case "all":
		return 0
	case "mapped":
		return 1
	case "unmapped":
		return 2
	default:
		return 0
	}
}

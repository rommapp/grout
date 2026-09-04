package ui

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"grout/catalog"
	"grout/cfw"
	"grout/files"
	"grout/romm"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type PlatformMappingInput struct {
	Host             settings.Host
	ApiTimeout       time.Duration
	CFW              cfw.CFW
	RomDirectory     string
	AutoSelect       bool
	HideBackButton   bool
	ExistingMappings map[string]settings.DirectoryMapping // For return visits, use existing config
	PlatformsBinding map[string]string                    // fs_slug -> bound slug for CFW lookups
}

type PlatformMappingOutput struct {
	Action   PlatformMappingAction
	Mappings map[string]settings.DirectoryMapping
}

type PlatformMappingScreen struct{}

func NewPlatformMappingScreen() *PlatformMappingScreen {
	return &PlatformMappingScreen{}
}

// Filter rows are read back by these keys rather than by their rendered labels,
// which change with the language.
const (
	filterKeyStatus     = "status"
	filterKeyWithGames  = "with_games"
	filterKeyCategory   = "category"
	filterKeyFamily     = "family"
	filterKeyGeneration = "generation"
)

// placeholderSlug marks the row shown when no platform survives the filter. It
// is not a platform, so it never reaches the saved mappings.
const placeholderSlug = "no_results"

func (s *PlatformMappingScreen) Draw(input PlatformMappingInput) (PlatformMappingOutput, error) {
	logger := gaba.GetLogger()
	output := PlatformMappingOutput{
		Action:   PlatformMappingActionBack,
		Mappings: make(map[string]settings.DirectoryMapping),
	}

	platforms, err := catalog.AllPlatforms(input.Host, input.ApiTimeout)
	if err != nil {
		logger.Error("Failed to load platforms", "error", err)
		return output, err
	}

	directories, err := files.SubdirectoryNames(input.RomDirectory)
	if err != nil {
		logger.Error("Failed to read ROM directory", "path", input.RomDirectory, "error", err)
		gaba.ConfirmationMessage(
			localize("platform_mapping_directory_not_found", "ROM Directory Could Not Be Found!"),
			[]gaba.FooterHelpItem{FooterBack()},
			gaba.MessageOptions{},
		)
		return output, fmt.Errorf("reading rom directory %s: %w", input.RomDirectory, err)
	}

	mappings := make(map[string]settings.DirectoryMapping, len(input.ExistingMappings))
	maps.Copy(mappings, input.ExistingMappings)

	// On a first run the auto-detected folders are captured up front, so a
	// platform the user never scrolled to still gets the folder grout picked
	// for it.
	if len(mappings) == 0 {
		for _, platform := range platforms {
			choices := s.choicesFor(platform, directories, mappings, input)
			if choices.Selected >= 0 {
				slug := platform.FSSlug
				mappings[slug] = settings.DirectoryMapping{
					RomMSlug:     slug,
					RelativePath: choices.Directories[choices.Selected].RelativePath,
				}
			}
		}
	}

	filter := catalog.PlatformFilter{Status: catalog.StatusAll}
	selectedIndex, visibleStartIndex := 0, 0

	for {
		items := s.buildItems(catalog.FilterPlatforms(platforms, filter, mappings), directories, mappings, input)
		if len(items) == 0 {
			items = []gaba.ItemWithOptions{noResultsItem()}
		}

		result, err := gaba.OptionsList(
			localize("platform_mapping_title", "Rom Directory Mapping"),
			gaba.OptionListSettings{
				InitialSelectedIndex:  selectedIndex,
				VisibleStartIndex:     visibleStartIndex,
				FooterHelpItems:       s.footer(input),
				DisableBackButton:     input.HideBackButton,
				StatusBar:             StatusBar(),
				ListPickerButton:      constants.VirtualButtonA,
				SecondaryActionButton: constants.VirtualButtonY,
			},
			items,
		)
		if err != nil {
			if errors.Is(err, gaba.ErrCancelled) {
				return PlatformMappingOutput{Action: PlatformMappingActionBack}, nil
			}
			return output, err
		}

		// Read the screen back before doing anything else, so changes survive a
		// trip through the filter menu.
		for _, item := range result.Items {
			slug, ok := item.Item.Metadata.(string)
			if !ok || slug == placeholderSlug {
				continue
			}
			path, _ := item.Options[item.SelectedOption].Value.(string)
			mappings[slug] = settings.DirectoryMapping{RomMSlug: slug, RelativePath: path}
		}

		if result.Action != gaba.ListActionSecondaryTriggered {
			break
		}

		var focused string
		if result.Selected >= 0 && result.Selected < len(items) {
			focused, _ = items[result.Selected].Item.Metadata.(string)
		}

		filter = s.filterMenu(platforms, filter)

		// Keep the cursor on the platform the user was looking at, wherever the
		// new filter puts it.
		next := s.buildItems(catalog.FilterPlatforms(platforms, filter, mappings), directories, mappings, input)
		selectedIndex = slices.IndexFunc(next, func(item gaba.ItemWithOptions) bool {
			slug, _ := item.Item.Metadata.(string)
			return focused != "" && slug == focused
		})
		if selectedIndex < 0 {
			selectedIndex = 0
		}
		visibleStartIndex = max(0, selectedIndex-(result.Selected-result.VisibleStartIndex))
	}

	for slug, mapping := range mappings {
		if mapping.RelativePath != "" {
			output.Mappings[slug] = mapping
		}
	}

	if err := cfw.CreateRomDirectories(output.Mappings, input.RomDirectory, directories); err != nil {
		logger.Error("Failed to create ROM directories", "error", err)
		return output, err
	}

	output.Action = PlatformMappingActionSaved
	return output, nil
}

func (s *PlatformMappingScreen) footer(input PlatformMappingInput) []gaba.FooterHelpItem {
	items := []gaba.FooterHelpItem{FooterCycle(), FooterSelect()}
	if !input.HideBackButton {
		items = slices.Insert(items, 0, FooterCancel())
	}
	return append(items,
		gaba.FooterHelpItem{ButtonName: "Y", HelpText: localize("button_filters", "Filters")},
		FooterSave(),
	)
}

func (s *PlatformMappingScreen) choicesFor(
	platform romm.Platform,
	directories []string,
	mappings map[string]settings.DirectoryMapping,
	input PlatformMappingInput,
) catalog.Choices {
	return catalog.DirectoryChoicesFor(catalog.ChoiceRequest{
		Platform:         platform,
		CFW:              input.CFW,
		Directories:      directories,
		PlatformsBinding: input.PlatformsBinding,
		Existing:         mappings,
		AutoSelect:       input.AutoSelect,
	})
}

func (s *PlatformMappingScreen) buildItems(
	platforms []romm.Platform,
	directories []string,
	mappings map[string]settings.DirectoryMapping,
	input PlatformMappingInput,
) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(platforms))
	for _, platform := range platforms {
		options, selected := platformOptions(s.choicesFor(platform, directories, mappings, input))
		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: platform.Name, Metadata: platform.FSSlug},
			Options:        options,
			SelectedOption: selected,
		})
	}
	return items
}

// platformOptions renders the folders a platform can map to: Skip, then the
// choices, then a free-text entry for a folder grout did not offer.
func platformOptions(choices catalog.Choices) ([]gaba.Option, int) {
	options := make([]gaba.Option, 0, len(choices.Directories)+2)
	options = append(options, gaba.Option{
		DisplayName: localize("common_skip", "Skip"),
		Value:       "",
	})

	for _, choice := range choices.Directories {
		id, fallback := "platform_mapping_path_prefix", "/{{.Name}}"
		if choice.Create {
			id, fallback = "platform_mapping_create", "Create '{{.Name}}'"
		}
		options = append(options, gaba.Option{
			DisplayName: localizeWith(id, fallback, map[string]any{"Name": choice.Display}),
			Value:       choice.RelativePath,
		})
	}

	selected := 0
	if choices.Selected >= 0 {
		selected = choices.Selected + 1
	}

	custom := gaba.Option{
		DisplayName: localize("platform_mapping_custom", "Custom..."),
		Value:       "",
		Type:        gaba.OptionTypeKeyboard,
	}
	if choices.Custom != "" {
		custom.DisplayName = choices.Custom
		custom.Value = choices.Custom
		custom.KeyboardPrompt = choices.Custom
		selected = len(options)
	}
	options = append(options, custom)

	return options, selected
}

func noResultsItem() gaba.ItemWithOptions {
	return gaba.ItemWithOptions{
		Item: gaba.MenuItem{
			Text:     localize("platform_mapping_no_results", "No matching platforms found. Press Y to filter."),
			Metadata: placeholderSlug,
		},
		Options: []gaba.Option{{DisplayName: "", Value: ""}},
	}
}

// filterMenu runs the filter sub-screen, returning the filter to apply.
// Cancelling leaves the current one in place.
func (s *PlatformMappingScreen) filterMenu(platforms []romm.Platform, current catalog.PlatformFilter) catalog.PlatformFilter {
	statusOptions := []gaba.Option{
		{DisplayName: localize("settings_mapping_status_all", "All"), Value: catalog.StatusAll},
		{DisplayName: localize("settings_mapping_status_mapped", "Mapped"), Value: catalog.StatusMapped},
		{DisplayName: localize("settings_mapping_status_unmapped", "Unmapped"), Value: catalog.StatusUnmapped},
	}
	gamesOptions := []gaba.Option{
		{DisplayName: localize("common_false", "False"), Value: false},
		{DisplayName: localize("common_true", "True"), Value: true},
	}

	// A metadata filter is shown only when RomM populated it. Category and
	// Family need IGDB metadata and are often empty, which would leave an
	// "All"-only picker that does nothing (#247).
	anyValue := string(catalog.StatusAll)
	categoryOptions := valueOptions(anyValue, catalog.Categories(platforms), func(c string) string { return c })
	familyOptions := valueOptions(anyValue, catalog.Families(platforms), func(f string) string { return f })
	generationOptions := valueOptions(0, catalog.Generations(platforms), func(g int) string {
		return fmt.Sprintf("Generation %d", g)
	})

	draft := current
	for {
		items := []gaba.ItemWithOptions{
			filterRow(filterKeyStatus, localize("settings_mapping_status", "Mapping Status"), statusOptions, draft.Status),
			filterRow(filterKeyWithGames, localize("settings_only_show_platforms_with_games", "Only Platforms with Games"), gamesOptions, draft.WithGamesOnly),
		}
		if len(categoryOptions) > 1 {
			items = append(items, filterRow(filterKeyCategory, localize("settings_category", "Category"), categoryOptions, draft.Category))
		}
		if len(familyOptions) > 1 {
			items = append(items, filterRow(filterKeyFamily, localize("settings_family", "Family"), familyOptions, draft.Family))
		}
		if len(generationOptions) > 1 {
			items = append(items, filterRow(filterKeyGeneration, localize("settings_generation", "Generation"), generationOptions, draft.Generation))
		}

		result, err := gaba.OptionsList(
			localize("game_filters_title", "Filters"),
			gaba.OptionListSettings{
				FooterHelpItems: []gaba.FooterHelpItem{
					FooterCancel(),
					FooterCycle(),
					{ButtonName: "X", HelpText: localize("button_reset", "Reset")},
					FooterSave(),
				},
				StatusBar:        StatusBar(),
				ListPickerButton: constants.VirtualButtonA,
				ActionButton:     constants.VirtualButtonX,
				UseSmallTitle:    true,
			},
			items,
		)
		if err != nil {
			return current
		}

		// Reset redraws the menu at its defaults; nothing applies until Save.
		if result.Action == gaba.ListActionTriggered {
			draft = catalog.PlatformFilter{Status: catalog.StatusAll}
			continue
		}

		return filterFrom(result.Items, draft)
	}
}

// valueOptions builds a picker that leads with All, which stores anyValue. A
// result of one entry means there was nothing to choose from.
func valueOptions[T any](anyValue any, values []T, describe func(T) string) []gaba.Option {
	options := make([]gaba.Option, 0, len(values)+1)
	options = append(options, gaba.Option{DisplayName: localize("filter_all", "All"), Value: anyValue})
	for _, value := range values {
		options = append(options, gaba.Option{DisplayName: describe(value), Value: value})
	}
	return options
}

func filterRow(key, label string, options []gaba.Option, current any) gaba.ItemWithOptions {
	return gaba.ItemWithOptions{
		Item:           gaba.MenuItem{Text: label, Metadata: key},
		Options:        options,
		SelectedOption: optionIndex(options, current),
	}
}

// optionIndex finds the option holding value, falling back to the first, which
// is always All.
func optionIndex(options []gaba.Option, value any) int {
	for i, option := range options {
		if option.Value == value {
			return i
		}
	}
	return 0
}

func filterFrom(items []gaba.ItemWithOptions, base catalog.PlatformFilter) catalog.PlatformFilter {
	filter := base
	for _, item := range items {
		value := item.Options[item.SelectedOption].Value
		switch item.Item.Metadata {
		case filterKeyStatus:
			if v, ok := value.(catalog.MappingStatus); ok {
				filter.Status = v
			}
		case filterKeyWithGames:
			if v, ok := value.(bool); ok {
				filter.WithGamesOnly = v
			}
		case filterKeyCategory:
			if v, ok := value.(string); ok {
				filter.Category = v
			}
		case filterKeyFamily:
			if v, ok := value.(string); ok {
				filter.Family = v
			}
		case filterKeyGeneration:
			if v, ok := value.(int); ok {
				filter.Generation = v
			}
		}
	}
	return filter
}

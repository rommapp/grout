package ui

import (
	"errors"
	"grout/cache"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	gabaconst "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type GameFiltersInput struct {
	Platform       romm.Platform
	CurrentFilters cache.GameFilter
	SearchQuery    string
}

type GameFiltersOutput struct {
	Action   GameFiltersAction
	Platform romm.Platform
	Filters  cache.GameFilter
}

type GameFiltersScreen struct{}

func NewGameFiltersScreen() *GameFiltersScreen {
	return &GameFiltersScreen{}
}

type filterCategory struct {
	labelID       string
	labelDefault  string
	lookupTable   string
	junctionTable string
	fkCol         string
}

var filterCategories = []filterCategory{
	{"filter_genre", "Genre", "genres", "game_genres", "genre_id"},
	{"filter_franchise", "Franchise", "franchises", "game_franchises", "franchise_id"},
	{"filter_company", "Company", "companies", "game_companies", "company_id"},
	{"filter_game_mode", "Game Mode", "game_modes", "game_game_modes", "game_mode_id"},
	{"filter_region", "Region", "regions", "game_regions", "region_id"},
	{"filter_language", "Language", "languages", "game_languages", "language_id"},
	{"filter_age_rating", "Age Rating", "age_ratings", "game_age_ratings", "age_rating_id"},
	{"filter_tag", "Tag", "tags", "game_tags", "tag_id"},
}

func (s *GameFiltersScreen) Draw(input GameFiltersInput) (GameFiltersOutput, error) {
	output := GameFiltersOutput{
		Action:   GameFiltersActionCancel,
		Platform: input.Platform,
		Filters:  input.CurrentFilters,
	}

	cm := cache.GetCacheManager()
	if cm == nil {
		return output, nil
	}

	platformID := input.Platform.ID
	items := s.buildMenuItems(cm, platformID, input.CurrentFilters, input.SearchQuery)

	if len(items) == 0 {
		return output, nil
	}

	result, err := gaba.OptionsList(
		i18n.Localize(&goi18n.Message{ID: "game_filters_title", Other: "Filters"}, nil),
		gaba.OptionListSettings{
			FooterHelpItems:  OptionsListFooter(),
			StatusBar:        StatusBar(),
			UseSmallTitle:    true,
			ListPickerButton: gabaconst.VirtualButtonA,
		},
		items,
	)

	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			return output, nil
		}
		return output, err
	}

	output.Filters = s.applyFilters(result.Items)
	output.Filters.PlatformID = platformID
	output.Action = GameFiltersActionApply
	return output, nil
}

func (s *GameFiltersScreen) buildMenuItems(cm *cache.Manager, platformID int, current cache.GameFilter, searchQuery string) []gaba.ItemWithOptions {
	currentValues := [8][]string{
		current.Genres, current.Franchises, current.Companies, current.GameModes,
		current.Regions, current.Languages, current.AgeRatings, current.Tags,
	}

	allLabel := i18n.Localize(&goi18n.Message{ID: "filter_all", Other: "All"}, nil)

	var items []gaba.ItemWithOptions
	var activeCats []int

	searchFilter := cache.GameFilter{SearchQuery: searchQuery}
	for catIdx, cat := range filterCategories {
		available := safeDistinct(cm.GetDistinctValuesWithFilter(cat.lookupTable, cat.junctionTable, cat.fkCol, platformID, searchFilter))
		if len(available) == 0 {
			continue
		}

		options := buildFilterOptionsList(allLabel, available, nil)

		selected := 0
		if len(currentValues[catIdx]) == 1 {
			for i, opt := range options {
				if v, ok := opt.Value.(string); ok && v == currentValues[catIdx][0] {
					selected = i
					break
				}
			}
		}

		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: i18n.Localize(&goi18n.Message{ID: cat.labelID, Other: cat.labelDefault}, nil)},
			Options:        options,
			SelectedOption: selected,
		})
		activeCats = append(activeCats, catIdx)
	}

	wireFilterCallbacks(cm, platformID, items, activeCats, allLabel, searchQuery)

	return items
}

func buildFilterOptionsList(allLabel string, available []string, onUpdate func(any)) []gaba.Option {
	options := make([]gaba.Option, 0, len(available)+1)
	options = append(options, gaba.Option{DisplayName: allLabel, Value: "", OnUpdate: onUpdate})
	for _, val := range available {
		options = append(options, gaba.Option{DisplayName: val, Value: val, OnUpdate: onUpdate})
	}
	return options
}

func wireFilterCallbacks(cm *cache.Manager, platformID int, items []gaba.ItemWithOptions, activeCats []int, allLabel string, searchQuery string) {
	if len(items) <= 1 {
		return
	}

	var makeCallback func(itemIdx int) func(any)
	makeCallback = func(itemIdx int) func(any) {
		return func(_ any) {
			filter := buildGameFilterFromSelections(items, activeCats)
			filter.SearchQuery = searchQuery

			for j := range items {
				if j == itemIdx {
					continue
				}

				cat := filterCategories[activeCats[j]]

				partialFilter := clearFilter(filter, activeCats[j])
				available := safeDistinct(cm.GetDistinctValuesWithFilter(
					cat.lookupTable, cat.junctionTable, cat.fkCol, platformID, partialFilter,
				))

				currentVal := ""
				if items[j].SelectedOption < len(items[j].Options) {
					if v, ok := items[j].Options[items[j].SelectedOption].Value.(string); ok {
						currentVal = v
					}
				}

				cb := makeCallback(j)
				options := buildFilterOptionsList(allLabel, available, cb)

				newSelected := 0
				if currentVal != "" {
					for i, opt := range options {
						if v, ok := opt.Value.(string); ok && v == currentVal {
							newSelected = i
							break
						}
					}
				}

				items[j].Options = options
				items[j].SelectedOption = newSelected
			}
		}
	}

	for i := range items {
		cb := makeCallback(i)
		for j := range items[i].Options {
			items[i].Options[j].OnUpdate = cb
		}
	}
}

func buildGameFilterFromSelections(items []gaba.ItemWithOptions, activeCats []int) cache.GameFilter {
	var f cache.GameFilter
	for i, catIdx := range activeCats {
		if items[i].SelectedOption >= len(items[i].Options) {
			continue
		}
		val, ok := items[i].Options[items[i].SelectedOption].Value.(string)
		if !ok || val == "" {
			continue
		}
		setGameFilter(&f, catIdx, val)
	}
	return f
}

func clearFilter(f cache.GameFilter, catIdx int) cache.GameFilter {
	switch catIdx {
	case 0:
		f.Genres = nil
	case 1:
		f.Franchises = nil
	case 2:
		f.Companies = nil
	case 3:
		f.GameModes = nil
	case 4:
		f.Regions = nil
	case 5:
		f.Languages = nil
	case 6:
		f.AgeRatings = nil
	case 7:
		f.Tags = nil
	}
	return f
}

func setGameFilter(f *cache.GameFilter, catIdx int, val string) {
	switch catIdx {
	case 0:
		f.Genres = []string{val}
	case 1:
		f.Franchises = []string{val}
	case 2:
		f.Companies = []string{val}
	case 3:
		f.GameModes = []string{val}
	case 4:
		f.Regions = []string{val}
	case 5:
		f.Languages = []string{val}
	case 6:
		f.AgeRatings = []string{val}
	case 7:
		f.Tags = []string{val}
	}
}

func (s *GameFiltersScreen) applyFilters(items []gaba.ItemWithOptions) cache.GameFilter {
	var f cache.GameFilter

	for _, item := range items {
		val, ok := item.Options[item.SelectedOption].Value.(string)
		if !ok || val == "" {
			continue
		}
		values := []string{val}

		text := item.Item.Text
		switch text {
		case i18n.Localize(&goi18n.Message{ID: "filter_genre", Other: "Genre"}, nil):
			f.Genres = values
		case i18n.Localize(&goi18n.Message{ID: "filter_franchise", Other: "Franchise"}, nil):
			f.Franchises = values
		case i18n.Localize(&goi18n.Message{ID: "filter_company", Other: "Company"}, nil):
			f.Companies = values
		case i18n.Localize(&goi18n.Message{ID: "filter_game_mode", Other: "Game Mode"}, nil):
			f.GameModes = values
		case i18n.Localize(&goi18n.Message{ID: "filter_region", Other: "Region"}, nil):
			f.Regions = values
		case i18n.Localize(&goi18n.Message{ID: "filter_language", Other: "Language"}, nil):
			f.Languages = values
		case i18n.Localize(&goi18n.Message{ID: "filter_age_rating", Other: "Age Rating"}, nil):
			f.AgeRatings = values
		case i18n.Localize(&goi18n.Message{ID: "filter_tag", Other: "Tag"}, nil):
			f.Tags = values
		}
	}

	return f
}

func safeDistinct(vals []string, _ error) []string {
	return vals
}

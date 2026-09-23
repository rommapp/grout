package ui

import (
	"errors"

	"grout/cache"
	"grout/catalog"
	"grout/romm"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	gabaconst "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type GameFiltersInput struct {
	Platform       romm.Platform
	Collection     romm.Collection
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

// filterCategory is one row of the filters screen: what it is called, where
// the cache keeps the values it can take, and which field of a GameFilter it
// fills.
//
// Reading a row back, narrowing the others, and restoring what was chosen last
// time all go through get and set, so a category is described once.
type filterCategory struct {
	// key identifies the row when the screen is read back, since a label
	// changes with the language.
	key          string
	labelID      string
	labelDefault string
	// lookupTable and its companions name where the cache holds this
	// category's values. The platform row leaves them empty: its values come
	// from the collection rather than from a metadata table.
	lookupTable   string
	junctionTable string
	fkCol         string
	get           func(cache.GameFilter) []string
	set           func(*cache.GameFilter, []string)
}

var filterCategories = []filterCategory{
	{
		key: "platform", labelID: "filter_platform", labelDefault: "Platform",
		get: func(f cache.GameFilter) []string { return f.PlatformSlugs },
		set: func(f *cache.GameFilter, v []string) { f.PlatformSlugs = v },
	},
	{
		key: "genre", labelID: "filter_genre", labelDefault: "Genre",
		lookupTable: "genres", junctionTable: "game_genres", fkCol: "genre_id",
		get: func(f cache.GameFilter) []string { return f.Genres },
		set: func(f *cache.GameFilter, v []string) { f.Genres = v },
	},
	{
		key: "franchise", labelID: "filter_franchise", labelDefault: "Franchise",
		lookupTable: "franchises", junctionTable: "game_franchises", fkCol: "franchise_id",
		get: func(f cache.GameFilter) []string { return f.Franchises },
		set: func(f *cache.GameFilter, v []string) { f.Franchises = v },
	},
	{
		key: "company", labelID: "filter_company", labelDefault: "Company",
		lookupTable: "companies", junctionTable: "game_companies", fkCol: "company_id",
		get: func(f cache.GameFilter) []string { return f.Companies },
		set: func(f *cache.GameFilter, v []string) { f.Companies = v },
	},
	{
		key: "game_mode", labelID: "filter_game_mode", labelDefault: "Game Mode",
		lookupTable: "game_modes", junctionTable: "game_game_modes", fkCol: "game_mode_id",
		get: func(f cache.GameFilter) []string { return f.GameModes },
		set: func(f *cache.GameFilter, v []string) { f.GameModes = v },
	},
	{
		key: "region", labelID: "filter_region", labelDefault: "Region",
		lookupTable: "regions", junctionTable: "game_regions", fkCol: "region_id",
		get: func(f cache.GameFilter) []string { return f.Regions },
		set: func(f *cache.GameFilter, v []string) { f.Regions = v },
	},
	{
		key: "language", labelID: "filter_language", labelDefault: "Language",
		lookupTable: "languages", junctionTable: "game_languages", fkCol: "language_id",
		get: func(f cache.GameFilter) []string { return f.Languages },
		set: func(f *cache.GameFilter, v []string) { f.Languages = v },
	},
	{
		key: "age_rating", labelID: "filter_age_rating", labelDefault: "Age Rating",
		lookupTable: "age_ratings", junctionTable: "game_age_ratings", fkCol: "age_rating_id",
		get: func(f cache.GameFilter) []string { return f.AgeRatings },
		set: func(f *cache.GameFilter, v []string) { f.AgeRatings = v },
	},
	{
		key: "tag", labelID: "filter_tag", labelDefault: "Tag",
		lookupTable: "tags", junctionTable: "game_tags", fkCol: "tag_id",
		get: func(f cache.GameFilter) []string { return f.Tags },
		set: func(f *cache.GameFilter, v []string) { f.Tags = v },
	},
}

// isPlatform reports whether this row lists platforms rather than metadata,
// which the cache answers a different way.
func (c filterCategory) isPlatform() bool { return c.lookupTable == "" }

// filterSource is what the rows are read out of.
type filterSource struct {
	manager    *cache.Manager
	platformID int
	collection romm.Collection
	// unified is a collection with no platform picked, which is the only case
	// where filtering by platform means anything.
	unified bool
}

func (s *GameFiltersScreen) Draw(input GameFiltersInput) (GameFiltersOutput, error) {
	output := GameFiltersOutput{
		Action:   GameFiltersActionCancel,
		Platform: input.Platform,
		Filters:  input.CurrentFilters,
	}

	manager := cache.GetCacheManager()
	if manager == nil {
		return output, nil
	}

	source := filterSource{
		manager:    manager,
		platformID: input.Platform.ID,
		collection: input.Collection,
		unified:    catalog.IsCollection(input.Collection) && input.Platform.ID == 0,
	}

	base := cache.GameFilter{NameSearch: input.SearchQuery}
	if catalog.IsCollection(input.Collection) {
		if id, err := manager.ResolveCollectionID(input.Collection); err == nil {
			base.CollectionInternalID = id
		}
	}

	rows, items := s.buildItems(source, base, input.CurrentFilters)
	if len(items) == 0 {
		return output, nil
	}
	wireNarrowing(source, base, rows, items)

	result, err := gaba.OptionsList(
		localize("game_filters_title", "Filters"),
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

	// Everything the screen narrowed by has to come back out, or a filtered
	// collection would be queried across the whole library.
	filters := selectedFilter(result.Items)
	filters.NameSearch = base.NameSearch
	filters.CollectionInternalID = base.CollectionInternalID
	filters.PlatformID = source.platformID

	output.Filters = filters
	output.Action = GameFiltersActionApply
	return output, nil
}

// buildItems makes a row for every category that has something to choose
// between, and returns the categories alongside so the rows can be read back.
func (s *GameFiltersScreen) buildItems(source filterSource, base cache.GameFilter, current cache.GameFilter) ([]filterCategory, []gaba.ItemWithOptions) {
	var rows []filterCategory
	var items []gaba.ItemWithOptions

	for _, category := range filterCategories {
		if category.isPlatform() && !source.unified {
			continue
		}

		options := category.options(source, base)
		// One entry is the "All" row on its own, which filters nothing.
		if len(options) <= 1 {
			continue
		}

		rows = append(rows, category)
		items = append(items, gaba.ItemWithOptions{
			Item:           gaba.MenuItem{Text: localize(category.labelID, category.labelDefault), Metadata: category.key},
			Options:        options,
			SelectedOption: optionIndex(options, chosenValue(category.get(current))),
		})
	}

	return rows, items
}

// chosenValue is the single value a category is filtered by, or "" for none.
// The screen offers one at a time even though a filter can hold several.
func chosenValue(values []string) string {
	if len(values) == 1 {
		return values[0]
	}
	return ""
}

// options lists what this category can be narrowed to, given everything else
// already chosen. The leading entry clears it.
func (c filterCategory) options(source filterSource, narrowed cache.GameFilter) []gaba.Option {
	all := gaba.Option{DisplayName: localize("filter_all", "All"), Value: ""}

	if c.isPlatform() {
		platforms, err := source.manager.GetCollectionPlatforms(source.collection, narrowed)
		if err != nil {
			gaba.GetLogger().Debug("Cannot list a collection's platforms to filter by", "error", err)
			return nil
		}

		options := make([]gaba.Option, 0, len(platforms)+1)
		options = append(options, all)
		for _, platform := range platforms {
			options = append(options, gaba.Option{DisplayName: platform.DisplayName, Value: platform.Slug})
		}
		return options
	}

	values, err := source.manager.GetDistinctValuesWithFilter(c.lookupTable, c.junctionTable, c.fkCol, source.platformID, narrowed)
	if err != nil {
		// The row is dropped rather than shown empty, so say why: silently
		// missing filters look like the library has no such metadata.
		gaba.GetLogger().Debug("Cannot list a filter's values", "category", c.key, "error", err)
		return nil
	}

	options := make([]gaba.Option, 0, len(values)+1)
	options = append(options, all)
	for _, value := range values {
		options = append(options, gaba.Option{DisplayName: value, Value: value})
	}
	return options
}

// selectedFilter reads the screen back. Rows are found by key, so a category
// that reads the same as another in some language cannot be mistaken for it.
func selectedFilter(items []gaba.ItemWithOptions) cache.GameFilter {
	byKey := make(map[string]filterCategory, len(filterCategories))
	for _, category := range filterCategories {
		byKey[category.key] = category
	}

	var filter cache.GameFilter
	for _, item := range items {
		key, _ := item.Item.Metadata.(string)
		category, ok := byKey[key]
		if !ok {
			continue
		}
		if value := selectedValue(item); value != "" {
			category.set(&filter, []string{value})
		}
	}
	return filter
}

func selectedValue(item gaba.ItemWithOptions) string {
	if item.SelectedOption < 0 || item.SelectedOption >= len(item.Options) {
		return ""
	}
	value, _ := item.Options[item.SelectedOption].Value.(string)
	return value
}

// wireNarrowing makes each row re-read the others whenever it changes, so the
// screen only ever offers combinations that match something.
//
// A row is narrowed by every choice except its own: including it would leave
// each row offering nothing but what it already shows.
func wireNarrowing(source filterSource, base cache.GameFilter, rows []filterCategory, items []gaba.ItemWithOptions) {
	if len(items) <= 1 {
		return
	}

	chosen := func() cache.GameFilter {
		filter := selectedFilter(items)
		filter.NameSearch = base.NameSearch
		filter.CollectionInternalID = base.CollectionInternalID
		return filter
	}

	var callbackFor func(int) func(any)
	callbackFor = func(changed int) func(any) {
		return func(any) {
			filter := chosen()

			for i := range items {
				if i == changed {
					continue
				}

				withoutOwn := filter
				rows[i].set(&withoutOwn, nil)

				options := rows[i].options(source, withoutOwn)
				if len(options) == 0 {
					continue
				}
				for j := range options {
					options[j].OnUpdate = callbackFor(i)
				}
				keepSelection(&items[i], options)

				// Narrowing a row can drop the value it held, which changes
				// what the rows after it should offer.
				filter = chosen()
			}
		}
	}

	for i := range items {
		callback := callbackFor(i)
		for j := range items[i].Options {
			items[i].Options[j].OnUpdate = callback
		}
	}
}

// keepSelection swaps in a narrowed set of options, staying on the same value
// when it survived and falling back to All when it did not.
func keepSelection(item *gaba.ItemWithOptions, options []gaba.Option) {
	current := selectedValue(*item)

	item.Options = options
	item.SelectedOption = 0
	if current != "" {
		item.SelectedOption = optionIndex(options, current)
	}
}

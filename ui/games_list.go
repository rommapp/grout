package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/catalog"
	"grout/environment"
	"grout/romm"
	"grout/settings"
	"strings"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	gabaconst "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	uatomic "go.uber.org/atomic"
)

type GameListApplied int

const (
	GameListAppliedNone GameListApplied = iota
	GameListAppliedSearch
	GameListAppliedFilters
)

type GameListInput struct {
	Config               *settings.Config
	Host                 settings.Host
	Platform             romm.Platform
	Collection           romm.Collection
	Games                []romm.Rom
	HasBIOS              bool
	SearchFilter         string
	GameFilter           cache.GameFilter
	LastApplied          GameListApplied
	LastSelectedIndex    int
	LastSelectedPosition int
}

type GameListOutput struct {
	Action               GameListAction
	SelectedGames        []romm.Rom
	Platform             romm.Platform
	Collection           romm.Collection
	SearchFilter         string
	GameFilter           cache.GameFilter
	LastApplied          GameListApplied
	AllGames             []romm.Rom
	HasBIOS              bool
	LastSelectedIndex    int
	LastSelectedPosition int
}

type GameListScreen struct{}

func NewGameListScreen() *GameListScreen {
	return &GameListScreen{}
}

func isCollectionSet(c romm.Collection) bool {
	return c.ID != 0 || c.VirtualID != ""
}

func (s *GameListScreen) Draw(input GameListInput) (GameListOutput, error) {
	games := input.Games
	hasBIOS := input.HasBIOS

	if len(games) == 0 {
		loaded, err := s.loadGames(input)
		if err != nil {
			s.showErrorMessage(err)
			return GameListOutput{Action: GameListActionBack}, nil
		}
		games = loaded.games
		hasBIOS = loaded.hasBIOS

		if input.Config.ShowBoxArt {
			go cache.SyncArtworkInBackground(input.Config.ArtKind, input.Host, games)
		}
	}

	output := GameListOutput{
		Action:               GameListActionBack,
		Platform:             input.Platform,
		Collection:           input.Collection,
		SearchFilter:         input.SearchFilter,
		GameFilter:           input.GameFilter,
		LastApplied:          input.LastApplied,
		AllGames:             games,
		HasBIOS:              hasBIOS,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	displayGames := prepareRomNames(games)

	if input.GameFilter.HasActiveFilters() {
		if cm := cache.GetCacheManager(); cm != nil {
			filter := input.GameFilter
			filter.PlatformID = input.Platform.ID
			if filtered, err := cm.GetFilteredGames(filter); err == nil {
				if isCollectionSet(input.Collection) {
					// Intersect: keep only collection games that match the filter
					allowed := make(map[int]struct{}, len(filtered))
					for _, g := range filtered {
						allowed[g.ID] = struct{}{}
					}
					kept := make([]romm.Rom, 0, len(displayGames))
					for _, g := range displayGames {
						if _, ok := allowed[g.ID]; ok {
							kept = append(kept, g)
						}
					}
					displayGames = kept
				} else {
					displayGames = prepareRomNames(filtered)
				}
			}
		}
	}

	if input.Config.DownloadedGames == settings.DownloadedGamesModeFilter {
		filteredGames := make([]romm.Rom, 0, len(displayGames))
		for _, game := range displayGames {
			if !isRomDownloaded(*input.Config, game) {
				filteredGames = append(filteredGames, game)
			}
		}
		displayGames = filteredGames
	}

	displayName := input.Platform.Name
	allGamesFilteredOut := false
	if isCollectionSet(input.Collection) {
		displayName = input.Collection.Name
		originalCount := len(displayGames)
		filteredGames := make([]romm.Rom, 0, len(displayGames))
		for _, game := range displayGames {
			if _, hasMapping := input.Config.DirectoryMappings[game.PlatformFSSlug]; hasMapping {
				filteredGames = append(filteredGames, game)
			}
		}
		displayGames = filteredGames

		allGamesFilteredOut = originalCount > 0 && len(displayGames) == 0

		if input.Platform.ID == 0 {
			for i := range displayGames {
				prefix := ""
				if input.Config.DownloadedGames == settings.DownloadedGamesModeMark && isRomDownloaded(*input.Config, displayGames[i]) {
					prefix = gabaconst.Download + " "
				}
				displayGames[i].DisplayName = fmt.Sprintf("%s[%s] %s", prefix, displayGames[i].PlatformFSSlug, displayGames[i].DisplayName)
			}
		} else {
			displayName = fmt.Sprintf("%s - %s", input.Collection.Name, input.Platform.Name)
			if input.Config.DownloadedGames == settings.DownloadedGamesModeMark {
				for i := range displayGames {
					if isRomDownloaded(*input.Config, displayGames[i]) {
						displayGames[i].DisplayName = fmt.Sprintf("%s %s", gabaconst.Download, displayGames[i].DisplayName)
					}
				}
			}
		}
	} else {
		for i := range displayGames {
			prefix := ""
			game := &displayGames[i]

			if game.HasNestedSingleFile {
				// For multi-file games, check if all files are downloaded
				allDownloaded := len(game.Files) > 0
				anyDownloaded := false
				for _, file := range game.Files {
					if isRomFileDownloaded(*input.Config, *game, file.FileName) {
						anyDownloaded = true
					} else {
						allDownloaded = false
					}
				}

				if input.Config.DownloadedGames == settings.DownloadedGamesModeMark {
					if allDownloaded {
						prefix = settings.MultipleDownloadedIcon + " "
					} else if anyDownloaded {
						prefix = gabaconst.Download + " "
					}
				}
				prefix += settings.MultipleFilesIcon + " "
			} else {
				if input.Config.DownloadedGames == settings.DownloadedGamesModeMark && isRomDownloaded(*input.Config, *game) {
					prefix = gabaconst.Download + " "
				}
			}

			if prefix != "" {
				game.DisplayName = prefix + game.DisplayName
			}
		}
	}

	title := displayName
	if input.GameFilter.HasActiveFilters() {
		filterLabel := i18n.Localize(&goi18n.Message{ID: "games_list_filtered", Other: "[Filtered]"}, nil)
		title = fmt.Sprintf("%s %s", filterLabel, title)
	}
	if input.SearchFilter != "" {
		message := i18n.Localize(&goi18n.Message{ID: "games_list_search_prefix", Other: "[Search: \"{{.Query}}\"]"}, map[string]interface{}{"Query": input.SearchFilter})
		title = fmt.Sprintf("%s %s", message, displayName)
		displayGames = catalog.FilterByName(displayGames, input.SearchFilter)
	}

	if len(displayGames) == 0 {
		if allGamesFilteredOut {
			s.showFilteredOutMessage(displayName)
		} else {
			s.showEmptyMessage(displayName, input.SearchFilter)
		}
		if clearLastFilter(&output, input.LastApplied) {
			return output, nil
		}
		output.Action = GameListActionBack
		return output, nil
	}

	menuItems := make([]gaba.MenuItem, len(displayGames))
	for i, game := range displayGames {
		imageFilename := ""
		if input.Config.ShowBoxArt {
			imageFilename = cache.GetArtworkCachePath(game.PlatformFSSlug, game.ID)
		}
		menuItems[i] = gaba.MenuItem{
			Text:          game.DisplayName,
			Selected:      false,
			Focused:       false,
			Metadata:      game,
			ImageFilename: imageFilename,
		}
	}

	// Only offer Filters when there's actually something to filter on: any loaded game
	// carries filterable metadata, or this is a unified collection (which offers a Platform
	// picker regardless). Otherwise the Filters screen would open empty and instantly close,
	// so we hide both the Y hint and the Y action rather than show a dead button.
	showFilters := catalog.HasFilterableMetadata(games) || (isCollectionSet(input.Collection) && input.Platform.ID == 0)

	options := gaba.DefaultListOptions(title, menuItems)
	options.UseSmallTitle = true
	options.ShowImages = input.Config.ShowBoxArt
	options.ActionButton = gabaconst.VirtualButtonX
	options.MultiSelectButton = gabaconst.VirtualButtonSelect
	options.DeselectAllButton = gabaconst.VirtualButtonL1
	options.SelectAllButton = gabaconst.VirtualButtonR1
	if showFilters {
		options.SecondaryActionButton = gabaconst.VirtualButtonY
	}

	options.OnL1 = func(selectedIndex int) int {
		if len(menuItems) == 0 {
			return selectedIndex
		}
		currentLetter := getLetter(menuItems[selectedIndex])
		firstIndexOfCurrent := selectedIndex
		for firstIndexOfCurrent > 0 && getLetter(menuItems[firstIndexOfCurrent-1]) == currentLetter {
			firstIndexOfCurrent--
		}
		if selectedIndex > firstIndexOfCurrent {
			return firstIndexOfCurrent
		}
		if firstIndexOfCurrent == 0 {
			return 0
		}
		prevLetter := getLetter(menuItems[firstIndexOfCurrent-1])
		prevIndex := firstIndexOfCurrent - 1
		for prevIndex > 0 && getLetter(menuItems[prevIndex-1]) == prevLetter {
			prevIndex--
		}
		return prevIndex
	}

	options.OnR1 = func(selectedIndex int) int {
		if len(menuItems) == 0 {
			return selectedIndex
		}
		currentLetter := getLetter(menuItems[selectedIndex])
		for i := selectedIndex + 1; i < len(menuItems); i++ {
			if getLetter(menuItems[i]) != currentLetter {
				return i
			}
		}
		return selectedIndex
	}

	if hasBIOS && !settings.IsKidModeEnabled() {
		options.TertiaryActionButton = gabaconst.VirtualButtonMenu
	}

	var footerItems []gaba.FooterHelpItem

	footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "B", HelpText: i18n.Localize(&goi18n.Message{ID: "button_back", Other: "Back"}, nil)})

	if hasBIOS && !settings.IsKidModeEnabled() {
		menuButtonName := i18n.Localize(&goi18n.Message{ID: "button_menu", Other: "Menu"}, nil)
		if environment.IsMiyoo() {
			menuButtonName = "L2"
		}
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: menuButtonName, HelpText: i18n.Localize(&goi18n.Message{ID: "button_bios", Other: "BIOS"}, nil)})
	}

	if showFilters {
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "Y", HelpText: i18n.Localize(&goi18n.Message{ID: "button_filters", Other: "Filters"}, nil), Group: gaba.FooterGroupRight})
	}

	footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_search", Other: "Search"}, nil), Group: gaba.FooterGroupRight})

	options.FooterHelpItems = footerItems

	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)
	options.StatusBar = StatusBar()

	res, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			if clearLastFilter(&output, input.LastApplied) {
				return output, nil
			}
			output.Action = GameListActionBack
			return output, nil
		}
		return output, err
	}

	switch res.Action {
	case gaba.ListActionSelected:
		selectedGames := make([]romm.Rom, 0, len(res.Selected))
		for _, idx := range res.Selected {
			selectedGames = append(selectedGames, res.Items[idx].Metadata.(romm.Rom))
		}
		output.LastSelectedIndex = res.Selected[0]
		output.LastSelectedPosition = res.VisiblePosition
		output.SelectedGames = selectedGames
		output.Action = GameListActionSelected
		return output, nil

	case gaba.ListActionTriggered:
		output.Action = GameListActionSearch
		return output, nil

	case gaba.ListActionSecondaryTriggered:
		output.LastSelectedIndex = res.Selected[0]
		output.LastSelectedPosition = res.VisiblePosition
		output.Action = GameListActionFilters
		return output, nil

	case gaba.ListActionTertiaryTriggered:
		output.Action = GameListActionBIOS
		return output, nil
	}

	output.Action = GameListActionBack
	return output, nil
}

type loadGamesResult struct {
	games   []romm.Rom
	hasBIOS bool
}

// loadGames reads the list, showing a progress dialog only when the cache
// misses and the server has to be asked.
func (s *GameListScreen) loadGames(input GameListInput) (loadGamesResult, error) {
	source := catalog.GameSource{Platform: input.Platform, Collection: input.Collection}

	if games, ok := catalog.CachedGames(source); ok {
		return loadGamesResult{games: games, hasBIOS: source.HasBIOS()}, nil
	}

	progress := uatomic.NewFloat64(0)
	var games []romm.Rom
	_, err := gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."},
			map[string]interface{}{"Name": source.Name()}),
		gaba.ProcessMessageOptions{
			ShowThemeBackground: true,
			ShowProgressBar:     !source.IsCollection(),
			Progress:            progress,
		},
		func() (interface{}, error) {
			var err error
			games, err = catalog.RefreshGames(source, progress)
			return nil, err
		},
	)
	if err != nil {
		return loadGamesResult{}, fmt.Errorf("failed to load games: %w", err)
	}

	return loadGamesResult{games: games, hasBIOS: source.HasBIOS()}, nil
}

func (s *GameListScreen) showEmptyMessage(platformName, searchFilter string) {
	var message string
	if searchFilter != "" {
		message = i18n.Localize(&goi18n.Message{ID: "games_list_no_results", Other: "No results found for \"{{.Query}}\""}, map[string]interface{}{"Query": searchFilter})
	} else {
		message = i18n.Localize(&goi18n.Message{ID: "games_list_no_games", Other: "No games found for {{.Name}}"}, map[string]interface{}{"Name": platformName})
	}

	gaba.ProcessMessage(
		message,
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			time.Sleep(time.Second * 1)
			return nil, nil
		},
	)
}

func (s *GameListScreen) showFilteredOutMessage(collectionName string) {
	message := i18n.Localize(&goi18n.Message{ID: "games_list_filtered_out", Other: "No games in {{.Name}} match your platform mappings"}, map[string]interface{}{"Name": collectionName})

	gaba.ProcessMessage(
		message,
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			time.Sleep(time.Second * 1)
			return nil, nil
		},
	)
}

func (s *GameListScreen) showErrorMessage(err error) {
	var message string

	classifiedErr := romm.ClassifyError(err)
	if errors.Is(classifiedErr, romm.ErrTimeout) {
		message = i18n.Localize(&goi18n.Message{ID: "games_list_load_timeout", Other: "Connection timed out!\nPlease check your network connection."}, nil)
	} else {
		message = i18n.Localize(&goi18n.Message{ID: "games_list_load_error", Other: "Failed to load games.\nPlease try again later."}, nil)
	}

	gaba.ProcessMessage(
		message,
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			time.Sleep(time.Second * 2)
			return nil, nil
		},
	)
}

func clearLastFilter(output *GameListOutput, lastApplied GameListApplied) bool {
	hasFilters := output.GameFilter.HasActiveFilters()
	hasSearch := output.SearchFilter != ""

	reset := func() {
		output.LastSelectedIndex = 0
		output.LastSelectedPosition = 0
		output.Action = GameListActionClearSearch
	}

	if lastApplied == GameListAppliedFilters && hasFilters {
		output.GameFilter = cache.GameFilter{}
		if hasSearch {
			output.LastApplied = GameListAppliedSearch
		} else {
			output.LastApplied = GameListAppliedNone
		}
		reset()
		return true
	}
	if lastApplied == GameListAppliedSearch && hasSearch {
		output.SearchFilter = ""
		if hasFilters {
			output.LastApplied = GameListAppliedFilters
		} else {
			output.LastApplied = GameListAppliedNone
		}
		reset()
		return true
	}

	if hasFilters {
		output.GameFilter = cache.GameFilter{}
		reset()
		return true
	}
	if hasSearch {
		output.SearchFilter = ""
		reset()
		return true
	}

	return false
}

func getLetter(item gaba.MenuItem) rune {
	if game, ok := item.Metadata.(romm.Rom); ok {
		name := strings.TrimSpace(game.Name)
		if len(name) == 0 {
			return '?'
		}
		return []rune(strings.ToUpper(name))[0]
	}
	return '?'
}

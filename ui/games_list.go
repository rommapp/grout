package ui

import (
	"errors"
	"fmt"
	"grout/constants"
	"grout/romm"
	"grout/utils"
	"slices"
	"strings"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type fetchType int

const (
	ftPlatform fetchType = iota
	ftCollection
)

type GameListInput struct {
	Config               *utils.Config
	Host                 romm.Host
	Platform             romm.Platform
	Collection           romm.Collection
	Games                []romm.Rom
	HasBIOS              bool
	SearchFilter         string
	LastSelectedIndex    int
	LastSelectedPosition int
}

type GameListOutput struct {
	SelectedGames        []romm.Rom
	Platform             romm.Platform
	Collection           romm.Collection
	SearchFilter         string
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

func (s *GameListScreen) Draw(input GameListInput) (ScreenResult[GameListOutput], error) {
	games := input.Games
	hasBIOS := input.HasBIOS

	if len(games) == 0 {
		loaded, err := s.loadGames(input)
		if err != nil {
			return withCode(GameListOutput{}, gaba.ExitCodeError), err
		}
		games = loaded.games
		hasBIOS = loaded.hasBIOS

		if input.Config.ShowBoxArt {
			go utils.SyncArtworkInBackground(input.Host, games)
		}
	}

	output := GameListOutput{
		Platform:             input.Platform,
		Collection:           input.Collection,
		SearchFilter:         input.SearchFilter,
		AllGames:             games,
		HasBIOS:              hasBIOS,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	displayGames := utils.PrepareRomNames(games, *input.Config)

	if input.Config.DownloadedGames == "filter" {
		filteredGames := make([]romm.Rom, 0, len(displayGames))
		for _, game := range displayGames {
			if !utils.IsGameDownloadedLocally(game, *input.Config) {
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
			if _, hasMapping := input.Config.DirectoryMappings[game.PlatformSlug]; hasMapping {
				filteredGames = append(filteredGames, game)
			}
		}
		displayGames = filteredGames

		allGamesFilteredOut = originalCount > 0 && len(displayGames) == 0

		if input.Platform.ID == 0 {
			for i := range displayGames {
				prefix := ""
				if input.Config.DownloadedGames == "mark" && utils.IsGameDownloadedLocally(displayGames[i], *input.Config) {
					prefix = utils.Downloaded + " "
				}
				displayGames[i].DisplayName = fmt.Sprintf("%s[%s] %s", prefix, displayGames[i].PlatformSlug, displayGames[i].DisplayName)
			}
		} else {
			displayName = fmt.Sprintf("%s - %s", input.Collection.Name, input.Platform.Name)
			if input.Config.DownloadedGames == "mark" {
				for i := range displayGames {
					if utils.IsGameDownloadedLocally(displayGames[i], *input.Config) {
						displayGames[i].DisplayName = fmt.Sprintf("%s %s", utils.Downloaded, displayGames[i].DisplayName)
					}
				}
			}
		}
	} else {
		if input.Config.DownloadedGames == "mark" {
			for i := range displayGames {
				if utils.IsGameDownloadedLocally(displayGames[i], *input.Config) {
					displayGames[i].DisplayName = fmt.Sprintf("%s %s", utils.Downloaded, displayGames[i].DisplayName)
				}
			}
		}
	}

	title := displayName
	if input.SearchFilter != "" {
		message := i18n.Localize(&goi18n.Message{ID: "games_list_search_prefix", Other: "[Search: \"{{.Query}}\"]"}, map[string]interface{}{"Query": input.SearchFilter})
		title = fmt.Sprintf("%s %s", message, displayName)
		displayGames = filterList(displayGames, input.SearchFilter)
	}

	if len(displayGames) == 0 {
		if allGamesFilteredOut {
			s.showFilteredOutMessage(displayName)
		} else {
			s.showEmptyMessage(displayName, input.SearchFilter)
		}
		if input.SearchFilter != "" {
			return withCode(output, constants.ExitCodeNoResults), nil
		}
		if isCollectionSet(input.Collection) && input.Platform.ID != 0 {
			return withCode(output, constants.ExitCodeBackToCollectionPlatform), nil
		}
		if isCollectionSet(input.Collection) {
			return withCode(output, constants.ExitCodeBackToCollection), nil
		}
		return back(output), nil
	}

	menuItems := make([]gaba.MenuItem, len(displayGames))
	for i, game := range displayGames {
		imageFilename := ""
		if input.Config.ShowBoxArt {
			imageFilename = utils.GetCachedArtworkForRom(game)
		}
		menuItems[i] = gaba.MenuItem{
			Text:          game.DisplayName,
			Selected:      false,
			Focused:       false,
			Metadata:      game,
			ImageFilename: imageFilename,
		}
	}

	options := gaba.DefaultListOptions(title, menuItems)
	options.SmallTitle = true
	options.EnableImages = input.Config.ShowBoxArt
	options.ActionButton = buttons.VirtualButtonX
	options.MultiSelectButton = buttons.VirtualButtonSelect
	options.HelpButton = buttons.VirtualButtonMenu

	if hasBIOS {
		options.SecondaryActionButton = buttons.VirtualButtonY
	}

	options.HelpTitle = i18n.Localize(&goi18n.Message{ID: "games_list_help_title", Other: "Games List Help"}, nil)
	options.HelpText = strings.Split(i18n.Localize(&goi18n.Message{ID: "games_list_help_body", Other: "A - Select a game\nB - Go back to the previous screen\nX - Search for games by name\nSelect - Toggle multi-select mode\n  In multi-select mode:\n  - Use D-Pad to navigate\n  - Press A to toggle selection\n  - Press Start to confirm selections\nMenu - Show this help screen\nD-Pad - Navigate the game list"}, nil), "\n")
	options.HelpExitText = i18n.Localize(&goi18n.Message{ID: "help_exit_text", Other: "Press any button to close help"}, nil)

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: i18n.Localize(&goi18n.Message{ID: "button_menu", Other: "Menu"}, nil), HelpText: i18n.Localize(&goi18n.Message{ID: "button_help", Other: "Help"}, nil)},
	}

	if hasBIOS {
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "Y", HelpText: i18n.Localize(&goi18n.Message{ID: "button_bios", Other: "BIOS"}, nil)})
	}

	footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_search", Other: "Search"}, nil)})

	options.FooterHelpItems = footerItems

	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)
	options.StatusBar = utils.StatusBar()

	res, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			if input.SearchFilter != "" {
				output.SearchFilter = ""
				output.LastSelectedIndex = 0
				output.LastSelectedPosition = 0
				return withCode(output, constants.ExitCodeClearSearch), nil
			}
			if isCollectionSet(input.Collection) && input.Platform.ID != 0 {
				return withCode(output, constants.ExitCodeBackToCollectionPlatform), nil
			}
			if isCollectionSet(input.Collection) {
				return withCode(output, constants.ExitCodeBackToCollection), nil
			}
			return back(output), nil
		}
		return withCode(output, gaba.ExitCodeError), err
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
		return success(output), nil

	case gaba.ListActionTriggered:
		return withCode(output, constants.ExitCodeSearch), nil

	case gaba.ListActionSecondaryTriggered:
		return withCode(output, constants.ExitCodeBIOS), nil
	}

	if isCollectionSet(input.Collection) && input.Platform.ID != 0 {
		return withCode(output, constants.ExitCodeBackToCollectionPlatform), nil
	}
	if isCollectionSet(input.Collection) {
		return withCode(output, constants.ExitCodeBackToCollection), nil
	}
	return back(output), nil
}

type loadGamesResult struct {
	games   []romm.Rom
	hasBIOS bool
}

func (s *GameListScreen) loadGames(input GameListInput) (loadGamesResult, error) {
	config := input.Config
	host := input.Host
	platform := input.Platform
	collection := input.Collection

	id := platform.ID
	ft := ftPlatform
	displayName := platform.Name

	if isCollectionSet(collection) {
		id = collection.ID
		ft = ftCollection
		displayName = collection.Name
	}

	logger := gaba.GetLogger()

	var result loadGamesResult

	// Check if we can use cached games (skip loading screen if so)
	cacheKey := getCacheKeyForFetch(id, ft)
	query := getQueryForFetch(id, ft)

	isFresh, _ := utils.CheckCacheFreshness(host, config, cacheKey, query)
	if isFresh {
		cached, err := utils.LoadCachedGames(cacheKey)
		if err == nil {
			logger.Debug("Loaded games from cache (no loading screen)", "key", cacheKey, "count", len(cached))
			result.games = cached

			// Check BIOS availability
			if platform.ID != 0 && !isCollectionSet(collection) {
				rc := utils.GetRommClient(host, config.ApiTimeout)
				firmware, err := rc.GetFirmware(platform.ID)
				if err == nil && len(firmware) > 0 {
					result.hasBIOS = true
				}
			}

			return result, nil
		}
	}

	// Cache miss or stale - show loading screen and fetch
	var loadErr error

	_, err := gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."}, map[string]interface{}{"Name": displayName}),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			rc := utils.GetRommClient(host, config.ApiTimeout)

			// Fetch games and BIOS info in parallel
			var wg sync.WaitGroup
			var gamesFetchErr error

			wg.Add(1)
			go func() {
				defer wg.Done()
				roms, err := fetchList(config, host, id, ft)
				if err != nil {
					logger.Error("Error downloading game list", "error", err)
					gamesFetchErr = err
					return
				}
				result.games = roms
			}()

			// Check BIOS availability (only for platforms, not collections)
			if platform.ID != 0 && !isCollectionSet(collection) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					firmware, err := rc.GetFirmware(platform.ID)
					if err == nil && len(firmware) > 0 {
						result.hasBIOS = true
					}
				}()
			}

			wg.Wait()

			if gamesFetchErr != nil {
				loadErr = gamesFetchErr
				return nil, gamesFetchErr
			}
			return nil, nil
		},
	)

	if err != nil || loadErr != nil {
		return loadGamesResult{}, fmt.Errorf("failed to load games: %w", err)
	}

	return result, nil
}

func getCacheKeyForFetch(id int, ft fetchType) string {
	switch ft {
	case ftPlatform:
		return utils.GetPlatformCacheKey(id)
	case ftCollection:
		return utils.GetCacheKey(utils.CacheTypeCollection, fmt.Sprintf("%d", id))
	}
	return ""
}

func getQueryForFetch(id int, ft fetchType) romm.GetRomsQuery {
	query := romm.GetRomsQuery{}
	switch ft {
	case ftPlatform:
		query.PlatformID = id
	case ftCollection:
		query.CollectionID = id
	}
	return query
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

const fetchPageSize = 1000

func fetchList(config *utils.Config, host romm.Host, queryID int, fetchType fetchType) ([]romm.Rom, error) {
	logger := gaba.GetLogger()

	// Build query for cache key and freshness check
	query := romm.GetRomsQuery{}
	var cacheKey string

	switch fetchType {
	case ftPlatform:
		query.PlatformID = queryID
		cacheKey = utils.GetPlatformCacheKey(queryID)
	case ftCollection:
		query.CollectionID = queryID
		cacheKey = utils.GetCacheKey(utils.CacheTypeCollection, fmt.Sprintf("%d", queryID))
	}

	// Check if cache is fresh
	isFresh, err := utils.CheckCacheFreshness(host, config, cacheKey, query)
	if err == nil && isFresh {
		// Load from cache
		cached, err := utils.LoadCachedGames(cacheKey)
		if err == nil {
			logger.Debug("Loaded games from cache", "key", cacheKey, "count", len(cached))
			return cached, nil
		}
		logger.Debug("Failed to load cached games, fetching fresh", "error", err)
	}

	// Fetch from API
	rc := utils.GetRommClient(host, config.ApiTimeout)

	var allGames []romm.Rom
	page := 1

	for {
		opt := romm.GetRomsQuery{
			Page:  page,
			Limit: fetchPageSize,
		}

		switch fetchType {
		case ftPlatform:
			opt.PlatformID = queryID
		case ftCollection:
			opt.CollectionID = queryID
		}

		res, err := rc.GetRoms(opt)
		if err != nil {
			return nil, err
		}

		allGames = append(allGames, res.Items...)
		logger.Debug("Fetched games page", "page", page, "count", len(res.Items), "total", res.Total, "fetched", len(allGames))

		// Check if we've fetched all items
		if len(allGames) >= res.Total || len(res.Items) == 0 {
			break
		}

		page++
	}

	logger.Debug("Fetched all games", "total", len(allGames))

	// Save to cache
	if err := utils.SaveGamesToCache(cacheKey, allGames); err != nil {
		logger.Debug("Failed to save games to cache", "error", err)
	}

	return allGames, nil
}

func filterList(itemList []romm.Rom, filter string) []romm.Rom {
	var result []romm.Rom

	for _, item := range itemList {
		if strings.Contains(strings.ToLower(item.Name), strings.ToLower(filter)) {
			result = append(result, item)
		}
	}

	slices.SortFunc(result, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return result
}

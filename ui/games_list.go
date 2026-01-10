package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/internal/constants"
	"grout/internal/stringutil"
	"grout/romm"
	"slices"
	"strings"
	"sync"
	"time"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	gabaconst "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	uatomic "go.uber.org/atomic"
)

type fetchType int

const (
	ftPlatform fetchType = iota
	ftCollection
)

type GameListInput struct {
	Config               *internal.Config
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
			s.showErrorMessage(err)
			return back(GameListOutput{}), nil
		}
		games = loaded.games
		hasBIOS = loaded.hasBIOS

		if input.Config.ShowBoxArt {
			go cache.SyncArtworkInBackground(input.Host, games)
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

	displayGames := stringutil.PrepareRomNames(games)

	if input.Config.DownloadedGames == "filter" {
		filteredGames := make([]romm.Rom, 0, len(displayGames))
		for _, game := range displayGames {
			if !game.IsDownloaded(*input.Config) {
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
				if input.Config.DownloadedGames == "mark" && displayGames[i].IsDownloaded(*input.Config) {
					prefix = gabaconst.Download + " "
				}
				displayGames[i].DisplayName = fmt.Sprintf("%s[%s] %s", prefix, displayGames[i].PlatformFSSlug, displayGames[i].DisplayName)
			}
		} else {
			displayName = fmt.Sprintf("%s - %s", input.Collection.Name, input.Platform.Name)
			if input.Config.DownloadedGames == "mark" {
				for i := range displayGames {
					if displayGames[i].IsDownloaded(*input.Config) {
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
					if game.IsFileDownloaded(*input.Config, file.FileName) {
						anyDownloaded = true
					} else {
						allDownloaded = false
					}
				}

				if input.Config.DownloadedGames == "mark" {
					if allDownloaded {
						prefix = constants.MultipleDownloaded + " "
					} else if anyDownloaded {
						prefix = gabaconst.Download + " "
					}
				}
				prefix += constants.MultipleFiles + " "
			} else {
				if input.Config.DownloadedGames == "mark" && game.IsDownloaded(*input.Config) {
					prefix = gabaconst.Download + " "
				}
			}

			if prefix != "" {
				game.DisplayName = prefix + game.DisplayName
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

	options := gaba.DefaultListOptions(title, menuItems)
	options.SmallTitle = true
	options.EnableImages = input.Config.ShowBoxArt
	options.ActionButton = gabaconst.VirtualButtonX
	options.MultiSelectButton = gabaconst.VirtualButtonSelect
	options.DeselectAllButton = gabaconst.VirtualButtonL1
	options.SelectAllButton = gabaconst.VirtualButtonR1
	options.HelpButton = gabaconst.VirtualButtonMenu

	if hasBIOS && !internal.IsKidModeEnabled() {
		options.SecondaryActionButton = gabaconst.VirtualButtonY
	}

	options.HelpTitle = i18n.Localize(&goi18n.Message{ID: "games_list_help_title", Other: "Games List Help"}, nil)
	options.HelpText = strings.Split(i18n.Localize(&goi18n.Message{ID: "games_list_help_body", Other: "A - Select a game\nB - Go back to the previous screen\nX - Search for games by name\nSelect - Toggle multi-select mode\n  In multi-select mode:\n  - Use D-Pad to navigate\n  - Press A to toggle selection\n  - Press L1 to deselect all\n  - Press R1 to select all\n  - Press Start to confirm selections\nMenu - Show this help screen\nD-Pad - Navigate the game list"}, nil), "\n")
	options.HelpExitText = i18n.Localize(&goi18n.Message{ID: "help_exit_text", Other: "Press any button to close help"}, nil)

	footerItems := []gaba.FooterHelpItem{
		{ButtonName: i18n.Localize(&goi18n.Message{ID: "button_menu", Other: "Menu"}, nil), HelpText: i18n.Localize(&goi18n.Message{ID: "button_help", Other: "Help"}, nil)},
	}

	if hasBIOS && !internal.IsKidModeEnabled() {
		footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "Y", HelpText: i18n.Localize(&goi18n.Message{ID: "button_bios", Other: "BIOS"}, nil)})
	}

	footerItems = append(footerItems, gaba.FooterHelpItem{ButtonName: "X", HelpText: i18n.Localize(&goi18n.Message{ID: "button_search", Other: "Search"}, nil)})

	options.FooterHelpItems = footerItems

	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)
	options.StatusBar = StatusBar()

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
	cm := cache.GetCacheManager()

	var result loadGamesResult

	// Check if we can use cached games (skip loading screen if so)
	if cm != nil {
		var cached []romm.Rom
		var err error

		if ft == ftPlatform {
			cached, err = cm.GetPlatformGames(id)
		} else if ft == ftCollection {
			cached, err = cm.GetCollectionGames(collection)
		}

		if err == nil && len(cached) > 0 {
			logger.Debug("Loaded games from cache (no loading screen)", "type", ft, "id", id, "count", len(cached))
			result.games = cached

			// Check BIOS availability from cached data
			if platform.ID != 0 && !isCollectionSet(collection) {
				if hasBIOS, wasFetched := cm.HasBIOS(platform.ID); wasFetched {
					result.hasBIOS = hasBIOS
				}
			}

			return result, nil
		}
	}

	// Cache miss or stale - show loading screen and fetch
	var loadErr error

	// For platforms, use progress bar since they can have many games
	if ft == ftPlatform && cm != nil {
		progress := uatomic.NewFloat64(0)
		_, err := gaba.ProcessMessage(
			i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."}, map[string]interface{}{"Name": displayName}),
			gaba.ProcessMessageOptions{
				ShowThemeBackground: true,
				ShowProgressBar:     true,
				Progress:            progress,
			},
			func() (interface{}, error) {
				rc := romm.NewClientFromHost(host, config.ApiTimeout)

				// Fetch games with progress and BIOS info in parallel
				var wg sync.WaitGroup
				var gamesFetchErr error

				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := cm.RefreshPlatformGamesWithProgress(platform, progress); err != nil {
						logger.Error("Failed to refresh platform games", "error", err)
						gamesFetchErr = err
						return
					}
					// Load from cache after refresh
					if games, err := cm.GetPlatformGames(id); err == nil {
						result.games = games
					} else {
						gamesFetchErr = err
					}
				}()

				// Check BIOS availability
				if hasBIOS, wasFetched := cm.HasBIOS(platform.ID); wasFetched {
					result.hasBIOS = hasBIOS
				} else {
					wg.Add(1)
					go func() {
						defer wg.Done()
						firmware, err := rc.GetFirmware(platform.ID)
						if err == nil && len(firmware) > 0 {
							result.hasBIOS = true
							cm.SetBIOSAvailability(platform.ID, true)
						} else {
							cm.SetBIOSAvailability(platform.ID, false)
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

	// For collections or when cache manager is unavailable, use simple loading screen
	_, err := gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "games_list_loading", Other: "Loading {{.Name}}..."}, map[string]interface{}{"Name": displayName}),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			rc := romm.NewClientFromHost(host, config.ApiTimeout)

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
				// First check cached BIOS info
				if cm := cache.GetCacheManager(); cm != nil {
					if hasBIOS, wasFetched := cm.HasBIOS(platform.ID); wasFetched {
						result.hasBIOS = hasBIOS
					} else {
						// Fall back to network fetch if not cached
						wg.Add(1)
						go func() {
							defer wg.Done()
							firmware, err := rc.GetFirmware(platform.ID)
							if err == nil && len(firmware) > 0 {
								result.hasBIOS = true
								// Cache the BIOS availability
								cm.SetBIOSAvailability(platform.ID, true)
							} else {
								cm.SetBIOSAvailability(platform.ID, false)
							}
						}()
					}
				} else {
					// No cache manager, do network fetch
					wg.Add(1)
					go func() {
						defer wg.Done()
						firmware, err := rc.GetFirmware(platform.ID)
						if err == nil && len(firmware) > 0 {
							result.hasBIOS = true
						}
					}()
				}
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

func fetchList(config *internal.Config, host romm.Host, queryID int, fetchType fetchType) ([]romm.Rom, error) {
	logger := gaba.GetLogger()
	cm := cache.GetCacheManager()

	switch fetchType {
	case ftPlatform:
		// Check cache first
		if cm != nil {
			if games, err := cm.GetPlatformGames(queryID); err == nil && len(games) > 0 {
				logger.Debug("Loaded platform games from cache", "platformID", queryID, "count", len(games))
				return games, nil
			}
		}

		// Cache miss - use efficient paginated fetch
		if cm != nil {
			platform := romm.Platform{ID: queryID}
			if err := cm.RefreshPlatformGames(platform); err != nil {
				logger.Error("Failed to refresh platform games", "error", err)
				return nil, err
			}
			// Load from cache after refresh
			if games, err := cm.GetPlatformGames(queryID); err == nil {
				logger.Debug("Loaded platform games after refresh", "platformID", queryID, "count", len(games))
				return games, nil
			}
		}

		// Cache manager should always be available - return error if not
		return nil, fmt.Errorf("cache manager not available")

	case ftCollection:
		// Collections should already be cached from initial population
		// This path shouldn't normally be hit since collection games are loaded via GetCollectionGames
		// with the full collection object. Return error if we get here without cache.
		return nil, fmt.Errorf("collection fetch requires cache manager")
	}

	return nil, fmt.Errorf("unsupported fetch type")
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

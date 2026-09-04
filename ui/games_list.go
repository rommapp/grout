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

	list := catalog.Browse(catalog.BrowseRequest{
		Config:     *input.Config,
		Games:      games,
		Platform:   input.Platform,
		Collection: input.Collection,
		Filter:     input.GameFilter,
		Search:     input.SearchFilter,
	})

	if len(list.Entries) == 0 {
		if list.AllMappedOut {
			s.showFilteredOutMessage(list.Title)
		} else {
			s.showEmptyMessage(list.Title, input.SearchFilter)
		}
		if clearLastFilter(&output, input.LastApplied) {
			return output, nil
		}
		output.Action = GameListActionBack
		return output, nil
	}

	title := listTitle(list.Title, input.GameFilter, input.SearchFilter)
	menuItems := menuItemsFor(list.Entries, *input.Config)

	// With nothing to filter on the Filters screen would open empty and close
	// again, so the Y hint and the Y action are hidden together. A unified
	// collection always has its platform picker to offer.
	showFilters := catalog.HasFilterableMetadata(games) || (catalog.IsCollection(input.Collection) && input.Platform.ID == 0)

	options := s.listOptions(title, menuItems, listChrome{
		Config:           *input.Config,
		ShowFilters:      showFilters,
		ShowBIOS:         hasBIOS && !settings.IsKidModeEnabled(),
		SelectedIndex:    input.LastSelectedIndex,
		SelectedPosition: input.LastSelectedPosition,
	})

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

// listTitle names what is on screen, saying up front when a search or a filter
// is the reason the list is short.
func listTitle(name string, filter cache.GameFilter, search string) string {
	if search != "" {
		prefix := localizeWith("games_list_search_prefix", `[Search: "{{.Query}}"]`, map[string]any{"Query": search})
		return prefix + " " + name
	}
	if filter.HasActiveFilters() {
		return localize("games_list_filtered", "[Filtered]") + " " + name
	}
	return name
}

func menuItemsFor(entries []catalog.GameEntry, config settings.Config) []gaba.MenuItem {
	items := make([]gaba.MenuItem, len(entries))
	for i, entry := range entries {
		image := ""
		if config.ShowBoxArt {
			image = cache.GetArtworkCachePath(entry.Game.PlatformFSSlug, entry.Game.ID)
		}
		items[i] = gaba.MenuItem{
			Text:          entryText(entry),
			Metadata:      entry.Game,
			ImageFilename: image,
		}
	}
	return items
}

// entryText puts the markers in front of a game's name: what is on the device
// already, and whether it holds more than one file.
func entryText(entry catalog.GameEntry) string {
	prefix := ""
	switch entry.Downloaded {
	case catalog.FullyDownloaded:
		if entry.MultipleFiles {
			prefix = settings.MultipleDownloadedIcon + " "
		} else {
			prefix = gabaconst.Download + " "
		}
	case catalog.PartlyDownloaded:
		prefix = gabaconst.Download + " "
	}

	if entry.MultipleFiles {
		prefix += settings.MultipleFilesIcon + " "
	}

	return prefix + entry.Name
}

// listChrome is what the games list offers beyond the games themselves.
type listChrome struct {
	Config      settings.Config
	ShowFilters bool
	// ShowBIOS gates the BIOS shortcut, which kid mode hides.
	ShowBIOS         bool
	SelectedIndex    int
	SelectedPosition int
}

func (s *GameListScreen) listOptions(title string, items []gaba.MenuItem, chrome listChrome) gaba.ListOptions {
	options := gaba.DefaultListOptions(title, items)
	options.UseSmallTitle = true
	options.ShowImages = chrome.Config.ShowBoxArt
	options.ActionButton = gabaconst.VirtualButtonX
	options.MultiSelectButton = gabaconst.VirtualButtonSelect
	options.DeselectAllButton = gabaconst.VirtualButtonL1
	options.SelectAllButton = gabaconst.VirtualButtonR1
	options.OnL1 = func(from int) int { return previousLetter(items, from) }
	options.OnR1 = func(from int) int { return nextLetter(items, from) }
	options.SelectedIndex = chrome.SelectedIndex
	options.VisibleStartIndex = max(0, chrome.SelectedIndex-chrome.SelectedPosition)
	options.StatusBar = StatusBar()

	if chrome.ShowFilters {
		options.SecondaryActionButton = gabaconst.VirtualButtonY
	}
	if chrome.ShowBIOS {
		options.TertiaryActionButton = gabaconst.VirtualButtonMenu
	}

	options.FooterHelpItems = gameListFooter(chrome)
	return options
}

func gameListFooter(chrome listChrome) []gaba.FooterHelpItem {
	items := []gaba.FooterHelpItem{FooterBack()}

	if chrome.ShowBIOS {
		// The Miyoo handhelds have no Menu button, so the shortcut sits on L2.
		name := localize("button_menu", "Menu")
		if environment.IsMiyoo() {
			name = "L2"
		}
		items = append(items, gaba.FooterHelpItem{ButtonName: name, HelpText: localize("button_bios", "BIOS")})
	}

	if chrome.ShowFilters {
		items = append(items, gaba.FooterHelpItem{
			ButtonName: "Y", HelpText: localize("button_filters", "Filters"), Group: gaba.FooterGroupRight,
		})
	}

	return append(items, gaba.FooterHelpItem{
		ButtonName: "X", HelpText: localize("button_search", "Search"), Group: gaba.FooterGroupRight,
	})
}

// previousLetter jumps to the start of the current initial, then to the start
// of the one before it, so holding L1 walks back through the alphabet.
func previousLetter(items []gaba.MenuItem, from int) int {
	if len(items) == 0 {
		return from
	}

	start := startOfLetter(items, from)
	if from > start || start == 0 {
		return start
	}
	return startOfLetter(items, start-1)
}

// nextLetter jumps to the first game filed under the next initial.
func nextLetter(items []gaba.MenuItem, from int) int {
	if len(items) == 0 {
		return from
	}

	letter := getLetter(items[from])
	for i := from + 1; i < len(items); i++ {
		if getLetter(items[i]) != letter {
			return i
		}
	}
	return from
}

// startOfLetter is the index of the first game sharing an initial with the one
// at index.
func startOfLetter(items []gaba.MenuItem, index int) int {
	letter := getLetter(items[index])
	for index > 0 && getLetter(items[index-1]) == letter {
		index--
	}
	return index
}

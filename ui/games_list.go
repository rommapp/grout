package ui

import (
	"errors"
	"fmt"
	"grout/models"
	"grout/utils"
	"path/filepath"
	"slices"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
	"github.com/brandonkowalski/go-romm"
)

// GameListInput contains data needed to render the game list screen
type GameListInput struct {
	Config               *models.Config
	Host                 models.Host
	Platform             romm.Platform
	Games                []romm.DetailedRom // Pre-loaded games (optional, will fetch if empty)
	SearchFilter         string
	LastSelectedIndex    int
	LastSelectedPosition int
}

// GameListOutput contains the result of the game list screen
type GameListOutput struct {
	SelectedGames        []romm.DetailedRom
	Platform             romm.Platform
	SearchFilter         string
	AllGames             []romm.DetailedRom // Full list for navigation back
	LastSelectedIndex    int
	LastSelectedPosition int
}

// GameListScreen displays a list of games for a platform
type GameListScreen struct{}

func NewGameListScreen() *GameListScreen {
	return &GameListScreen{}
}

func (s *GameListScreen) Draw(input GameListInput) (gaba.ScreenResult[GameListOutput], error) {
	games := input.Games

	if len(games) == 0 {
		loaded, err := s.loadGames(input.Config, input.Host, input.Platform)
		if err != nil {
			return gaba.ScreenResult[GameListOutput]{ExitCode: gaba.ExitCodeError}, err
		}
		games = loaded
	}

	output := GameListOutput{
		Platform:             input.Platform,
		SearchFilter:         input.SearchFilter,
		AllGames:             games,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	displayGames := s.prepareDisplayList(games)

	title := input.Platform.Name
	if input.SearchFilter != "" {
		title = fmt.Sprintf("[Search: \"%s\"] | %s", input.SearchFilter, input.Platform.Name)
		displayGames = filterList(displayGames, input.SearchFilter)
	}

	// Handle empty results
	if len(displayGames) == 0 {
		s.showEmptyMessage(input.Platform.Name, input.SearchFilter)
		return gaba.WithCode(output, gaba.ExitCode(404)), nil
	}

	menuItems := make([]gaba.MenuItem, len(displayGames))
	for i, game := range displayGames {
		menuItems[i] = gaba.MenuItem{
			Text:     game.Name,
			Selected: false,
			Focused:  false,
			Metadata: game,
		}
	}

	options := gaba.DefaultListOptions(title, menuItems)
	options.SmallTitle = true
	options.EnableAction = true
	options.EnableMultiSelect = true
	options.FooterHelpItems = []gaba.FooterHelpItem{
		{ButtonName: "B", HelpText: "Back"},
		{ButtonName: "X", HelpText: "Search"},
		{ButtonName: "Select", HelpText: "Multi"},
		{ButtonName: "A", HelpText: "Select"},
	}

	options.SelectedIndex = input.LastSelectedIndex
	options.VisibleStartIndex = max(0, input.LastSelectedIndex-input.LastSelectedPosition)

	res, err := gaba.List(options)
	if err != nil {
		if errors.Is(err, gaba.ErrCancelled) {
			if input.SearchFilter != "" {
				output.SearchFilter = ""
				output.LastSelectedIndex = 0
				output.LastSelectedPosition = 0
				return gaba.WithCode(output, utils.ExitCodeClearSearch), nil
			}
			return gaba.Back(output), nil
		}
		return gaba.WithCode(output, gaba.ExitCodeError), err
	}

	switch res.Action {
	case gaba.ListActionSelected:
		selectedGames := make([]romm.DetailedRom, 0, len(res.Selected))
		for _, idx := range res.Selected {
			selectedGames = append(selectedGames, res.Items[idx].Metadata.(romm.DetailedRom))
		}
		output.LastSelectedIndex = res.Selected[0]
		output.LastSelectedPosition = res.VisiblePosition
		output.SelectedGames = selectedGames
		return gaba.Success(output), nil

	case gaba.ListActionTriggered:
		return gaba.WithCode(output, gaba.ExitCodeSearch), nil
	}

	return gaba.Back(output), nil
}

func (s *GameListScreen) loadGames(config *models.Config, host models.Host, platform romm.Platform) ([]romm.DetailedRom, error) {
	logger := gaba.GetLogger()

	var games []romm.DetailedRom
	var loadErr error

	_, err := gaba.ProcessMessage(
		fmt.Sprintf("Loading %s...", platform.Name),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			roms, err := fetchList(config, host, platform)
			if err != nil {
				logger.Error("Error downloading game list", "error", err)
				loadErr = err
				return nil, err
			}
			games = roms
			return nil, nil
		},
	)

	if err != nil || loadErr != nil {
		return nil, fmt.Errorf("failed to load games: %w", err)
	}

	return games, nil
}

func (s *GameListScreen) prepareDisplayList(games []romm.DetailedRom) []romm.DetailedRom {
	for i := range games {
		if games[i].Name == "" {
			games[i].Name = strings.ReplaceAll(games[i].FileName, filepath.Ext(games[i].FileName), "")
		}
	}

	slices.SortFunc(games, func(a, b romm.DetailedRom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return games
}

func (s *GameListScreen) showEmptyMessage(platformName, searchFilter string) {
	var message string
	if searchFilter != "" {
		message = fmt.Sprintf("No results found for \"%s\"", searchFilter)
	} else {
		message = fmt.Sprintf("No games found for %s", platformName)
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

func fetchList(config *models.Config, host models.Host, platform romm.Platform) ([]romm.DetailedRom, error) {
	logger := gaba.GetLogger()

	logger.Debug("Fetching Item List",
		"host", host.ToLoggable())

	rc := romm.NewClient(host.URL(),
		romm.WithBasicAuth(host.Username, host.Password),
		romm.WithTimeout(config.ApiTimeout))

	res, err := rc.GetDetailedRoms(&romm.GetRomsOptions{
		Size:       10000,
		PlatformID: &platform.ID,
	})
	if err != nil {
		return nil, err
	}

	filtered := make([]romm.DetailedRom, 0, res.Size)
	for _, rom := range res.Items {
		if !strings.HasPrefix(rom.FileName, ".") {
			filtered = append(filtered, rom)
		}
	}

	return filtered, nil
}

func filterList(itemList []romm.DetailedRom, filter string) []romm.DetailedRom {
	var result []romm.DetailedRom

	for _, item := range itemList {
		if strings.Contains(strings.ToLower(item.Name), strings.ToLower(filter)) {
			result = append(result, item)
		}
	}

	slices.SortFunc(result, func(a, b romm.DetailedRom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return result
}

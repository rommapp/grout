package ui

import (
	"errors"
	"fmt"
	"grout/constants"
	"grout/models"
	"grout/romm"
	"grout/utils"
	"slices"
	"strings"
	"time"

	gaba "github.com/UncleJunVIP/gabagool/v2/pkg/gabagool"
)

type FetchType int

const (
	Platform FetchType = iota
	Collection
)

type GameListInput struct {
	Config               *models.Config
	Host                 models.Host
	Platform             romm.Platform
	Collection           romm.Collection
	Games                []romm.Rom
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
	LastSelectedIndex    int
	LastSelectedPosition int
}

type GameListScreen struct{}

func NewGameListScreen() *GameListScreen {
	return &GameListScreen{}
}

func (s *GameListScreen) Draw(input GameListInput) (ScreenResult[GameListOutput], error) {
	games := input.Games

	if len(games) == 0 {
		loaded, err := s.loadGames(input)
		if err != nil {
			return WithCode(GameListOutput{}, gaba.ExitCodeError), err
		}
		games = loaded
	}

	output := GameListOutput{
		Platform:             input.Platform,
		Collection:           input.Collection,
		SearchFilter:         input.SearchFilter,
		AllGames:             games,
		LastSelectedIndex:    input.LastSelectedIndex,
		LastSelectedPosition: input.LastSelectedPosition,
	}

	displayGames := utils.PrepareRomNames(games)

	displayName := input.Platform.Name
	allGamesFilteredOut := false
	if input.Collection.ID != 0 {
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

		// Only add platform prefix if we're viewing multiple platforms (no specific platform selected)
		if input.Platform.ID == 0 {
			for i := range displayGames {
				displayGames[i].ListName = fmt.Sprintf("[%s] %s", displayGames[i].PlatformSlug, displayGames[i].DisplayName)
			}
		} else {
			// Viewing a specific platform within collection, show platform name
			displayName = fmt.Sprintf("%s - %s", input.Collection.Name, input.Platform.Name)
		}
	}

	title := displayName
	if input.SearchFilter != "" {
		title = fmt.Sprintf("[Search: \"%s\"] | %s", input.SearchFilter, displayName)
		displayGames = filterList(displayGames, input.SearchFilter)
	}

	if len(displayGames) == 0 {
		if allGamesFilteredOut {
			s.showFilteredOutMessage(displayName)
		} else {
			s.showEmptyMessage(displayName, input.SearchFilter)
		}
		if input.SearchFilter != "" {
			return WithCode(output, constants.ExitCodeNoResults), nil
		}
		if input.Collection.ID != 0 && input.Platform.ID != 0 {
			return WithCode(output, constants.ExitCodeBackToCollectionPlatform), nil
		}
		if input.Collection.ID != 0 {
			return WithCode(output, constants.ExitCodeBackToCollection), nil
		}
		return Back(output), nil
	}

	menuItems := make([]gaba.MenuItem, len(displayGames))
	for i, game := range displayGames {
		menuItems[i] = gaba.MenuItem{
			Text:     game.ListName,
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
				return WithCode(output, constants.ExitCodeClearSearch), nil
			}
			// Return different exit code based on whether viewing collection or platform
			if input.Collection.ID != 0 && input.Platform.ID != 0 {
				return WithCode(output, constants.ExitCodeBackToCollectionPlatform), nil
			}
			if input.Collection.ID != 0 {
				return WithCode(output, constants.ExitCodeBackToCollection), nil
			}
			return Back(output), nil
		}
		return WithCode(output, gaba.ExitCodeError), err
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
		return Success(output), nil

	case gaba.ListActionTriggered:
		return WithCode(output, constants.ExitCodeSearch), nil
	}

	if input.Collection.ID != 0 && input.Platform.ID != 0 {
		return WithCode(output, constants.ExitCodeBackToCollectionPlatform), nil
	}
	if input.Collection.ID != 0 {
		return WithCode(output, constants.ExitCodeBackToCollection), nil
	}
	return Back(output), nil
}

func (s *GameListScreen) loadGames(input GameListInput) ([]romm.Rom, error) {
	config := input.Config
	host := input.Host
	platform := input.Platform
	collection := input.Collection

	id := platform.ID
	ft := Platform
	displayName := platform.Name

	if collection.ID != 0 {
		id = collection.ID
		ft = Collection
		displayName = collection.Name
	}

	logger := gaba.GetLogger()

	var games []romm.Rom
	var loadErr error

	_, err := gaba.ProcessMessage(
		fmt.Sprintf("Loading %s...", displayName),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			roms, err := fetchList(config, host, id, ft)
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

func (s *GameListScreen) showFilteredOutMessage(collectionName string) {
	message := fmt.Sprintf("No games in %s match your platform mappings", collectionName)

	gaba.ProcessMessage(
		message,
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (interface{}, error) {
			time.Sleep(time.Second * 1)
			return nil, nil
		},
	)
}

func fetchList(config *models.Config, host models.Host, queryID int, fetchType FetchType) ([]romm.Rom, error) {
	logger := gaba.GetLogger()

	rc := romm.NewClient(host.URL(),
		romm.WithBasicAuth(host.Username, host.Password),
		romm.WithTimeout(config.ApiTimeout))

	opt := &romm.GetRomsOptions{
		Limit: 10000,
	}

	switch fetchType {
	case Platform:
		opt.PlatformID = &queryID
	case Collection:
		opt.CollectionID = &queryID
	}

	res, err := rc.GetRoms(opt)
	if err != nil {
		return nil, err
	}
	logger.Debug("Fetched platform games", "count", len(res.Items), "total", res.Total)
	return res.Items, nil
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

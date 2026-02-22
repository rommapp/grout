package ui

import (
	"errors"
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/romm"
	"sort"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SyncedGamesInput struct {
	Config    *internal.Config
	Host      romm.Host
	Platforms *[]romm.Platform
	DeviceID  string
}

type SyncedGamesOutput struct {
	Action SyncedGamesAction
}

type SyncedGamesScreen struct{}

func NewSyncedGamesScreen() *SyncedGamesScreen {
	return &SyncedGamesScreen{}
}

func (s *SyncedGamesScreen) Draw(input SyncedGamesInput) (SyncedGamesOutput, error) {
	output := SyncedGamesOutput{Action: SyncedGamesActionBack}

	cm := cache.GetCacheManager()
	if cm == nil {
		return output, nil
	}

	romIDs := cm.GetSyncedRomIDs(input.DeviceID)
	if len(romIDs) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "synced_games_empty", Other: "No synced games found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	games, _ := cm.GetGamesByIDs(romIDs)
	if len(games) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "synced_games_empty", Other: "No synced games found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	// Build enabled-platform set from mapped platforms
	enabledSlugs := make(map[string]bool)
	if input.Platforms != nil {
		for _, p := range *input.Platforms {
			enabledSlugs[p.FSSlug] = true
		}
	}

	// Group games by PlatformFSSlug, keeping only enabled platforms
	type platformGroup struct {
		Name  string
		Slug  string
		Games []romm.Rom
	}
	groupMap := make(map[string]*platformGroup)

	for _, game := range games {
		slug := game.PlatformFSSlug
		if !enabledSlugs[slug] {
			continue
		}
		g, ok := groupMap[slug]
		if !ok {
			displayName := game.PlatformDisplayName
			if displayName == "" {
				displayName = slug
			}
			g = &platformGroup{Name: displayName, Slug: slug}
			groupMap[slug] = g
		}
		g.Games = append(g.Games, game)
	}

	if len(groupMap) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "synced_games_empty", Other: "No synced games found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	// Sort platforms alphabetically
	var groups []platformGroup
	for _, g := range groupMap {
		sort.Slice(g.Games, func(i, j int) bool {
			return g.Games[i].Name < g.Games[j].Name
		})
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	// Outer loop: platform list
	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout)
	platformIndex := 0
	platformVisibleStart := 0

	for {
		platformItems := make([]gaba.MenuItem, len(groups))
		for i, g := range groups {
			platformItems[i] = gaba.MenuItem{
				Text:     fmt.Sprintf("%s (%d)", g.Name, len(g.Games)),
				Metadata: i,
			}
		}

		options := gaba.DefaultListOptions(
			i18n.Localize(&goi18n.Message{ID: "synced_games_title", Other: "Synced Games"}, nil),
			platformItems,
		)
		options.FooterHelpItems = []gaba.FooterHelpItem{FooterBack(), FooterSelect()}
		options.SelectedIndex = platformIndex
		options.VisibleStartIndex = platformVisibleStart
		options.StatusBar = StatusBar()

		sel, err := gaba.List(options)
		if err != nil {
			if errors.Is(err, gaba.ErrCancelled) {
				return output, nil
			}
			return output, err
		}

		if sel.Action == gaba.ListActionSelected {
			platformIndex = sel.Selected[0]
			platformVisibleStart = max(0, sel.Selected[0]-sel.VisiblePosition)

			group := groups[platformIndex]
			s.showPlatformGames(client, input.Config, group.Name, group.Games)
			continue
		}

		return output, nil
	}
}

func (s *SyncedGamesScreen) showPlatformGames(client *romm.Client, config *internal.Config, platformName string, games []romm.Rom) {
	menuItems := make([]gaba.MenuItem, len(games))
	for i, game := range games {
		menuItems[i] = gaba.MenuItem{
			Text:     game.Name,
			Metadata: game.ID,
		}
	}

	selectedIndex := 0
	visibleStart := 0

	for {
		options := gaba.DefaultListOptions(platformName, menuItems)
		options.FooterHelpItems = []gaba.FooterHelpItem{FooterBack(), FooterSelect()}
		options.SelectedIndex = selectedIndex
		options.VisibleStartIndex = visibleStart
		options.StatusBar = StatusBar()

		sel, err := gaba.List(options)
		if err != nil {
			return
		}

		if sel.Action == gaba.ListActionSelected {
			selectedIndex = sel.Selected[0]
			visibleStart = max(0, sel.Selected[0]-sel.VisiblePosition)

			romID := menuItems[selectedIndex].Metadata.(int)
			s.showGameDetail(client, config, romID, menuItems[selectedIndex].Text)
			continue
		}

		return
	}
}

func (s *SyncedGamesScreen) showGameDetail(client *romm.Client, config *internal.Config, romID int, gameName string) {
	var summary romm.SaveSummary
	var fetchErr error
	gaba.ProcessMessage(
		i18n.Localize(&goi18n.Message{ID: "synced_games_loading_detail", Other: "Loading save details..."}, nil),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			summary, fetchErr = client.GetSaveSummary(romID)
			return nil, fetchErr
		},
	)

	if fetchErr != nil {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "synced_games_detail_error", Other: "Failed to load save details."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return
	}

	sections := s.buildDetailSections(config, romID, summary)

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ConfirmButton = buttons.VirtualButtonUnassigned

	gaba.DetailScreen(
		gameName,
		options,
		[]gaba.FooterHelpItem{FooterBack()},
	)
}

func (s *SyncedGamesScreen) buildDetailSections(config *internal.Config, romID int, summary romm.SaveSummary) []gaba.Section {
	var sections []gaba.Section

	slotPref := config.GetSlotPreference(romID)
	overviewItems := []gaba.MetadataItem{
		{
			Label: i18n.Localize(&goi18n.Message{ID: "synced_games_total_saves", Other: "Total Saves"}, nil),
			Value: fmt.Sprintf("%d", summary.TotalCount),
		},
		{
			Label: i18n.Localize(&goi18n.Message{ID: "synced_games_active_slot", Other: "Active Slot"}, nil),
			Value: slotPref,
		},
	}
	sections = append(sections, gaba.NewInfoSection(
		i18n.Localize(&goi18n.Message{ID: "synced_games_overview", Other: "Overview"}, nil),
		overviewItems,
	))

	for _, slot := range summary.Slots {
		slotName := "default"
		if slot.Slot != nil {
			slotName = *slot.Slot
		}

		slotItems := []gaba.MetadataItem{
			{
				Label: i18n.Localize(&goi18n.Message{ID: "synced_games_save_count", Other: "Save Count"}, nil),
				Value: fmt.Sprintf("%d", slot.Count),
			},
			{
				Label: i18n.Localize(&goi18n.Message{ID: "synced_games_latest_save", Other: "Latest Save"}, nil),
				Value: slot.Latest.UpdatedAt.Format("Jan 2, 2006 3:04 PM"),
			},
			{
				Label: i18n.Localize(&goi18n.Message{ID: "synced_games_latest_file", Other: "File"}, nil),
				Value: slot.Latest.FileName,
			},
		}

		sections = append(sections, gaba.NewInfoSection(slotName, slotItems))
	}

	return sections
}

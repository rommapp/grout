package ui

import (
	"errors"
	"fmt"

	"grout/cache"
	"grout/catalog"
	"grout/romm"
	"grout/saves"
	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
)

type SyncedGamesInput struct {
	Config    *settings.Config
	Host      settings.Host
	Platforms *[]romm.Platform
	DeviceID  string
}

type SyncedGamesOutput struct {
	Action       SyncedGamesAction
	Config       *settings.Config
	NewSlotName  string // Set when a new slot is created (for targeted upload)
	NewSlotRomID int    // ROM ID to upload saves for
}

type SyncedGamesScreen struct{}

func NewSyncedGamesScreen() *SyncedGamesScreen {
	return &SyncedGamesScreen{}
}

// slotChange is returned when the user picks a slot that means a sync should
// run. SlotName is set only for a slot the server does not hold yet, which has
// to be filled by uploading rather than synced.
type slotChange struct {
	SlotName string
	RomID    int
}

func (s *SyncedGamesScreen) Draw(input SyncedGamesInput) (SyncedGamesOutput, error) {
	output := SyncedGamesOutput{Action: SyncedGamesActionBack, Config: input.Config}

	groups := s.syncedByPlatform(input)
	if len(groups) == 0 {
		gaba.ConfirmationMessage(
			localize("synced_games_empty", "No synced games found."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	client := romm.NewClientFromHost(input.Host, input.Config.ApiTimeout.Duration())
	selected, visibleStart := 0, 0

	for {
		items := make([]gaba.MenuItem, len(groups))
		for i, group := range groups {
			items[i] = gaba.MenuItem{Text: fmt.Sprintf("%s (%d)", group.Name, len(group.Games))}
		}

		options := gaba.DefaultListOptions(localize("synced_games_title", "Synced Games"), items)
		options.FooterHelpItems = []gaba.FooterHelpItem{FooterBack(), FooterSelect()}
		options.SelectedIndex = selected
		options.VisibleStartIndex = visibleStart
		options.StatusBar = StatusBar()

		result, err := gaba.List(options)
		if err != nil {
			if errors.Is(err, gaba.ErrCancelled) {
				return output, nil
			}
			return output, err
		}
		if result.Action != gaba.ListActionSelected {
			return output, nil
		}

		selected = result.Selected[0]
		visibleStart = max(0, selected-result.VisiblePosition)

		group := groups[selected]
		if change := s.showGames(client, input.Config, group.Name, group.Games); change != nil {
			output.Action = SyncedGamesActionSyncNow
			output.NewSlotName = change.SlotName
			output.NewSlotRomID = change.RomID
			return output, nil
		}
	}
}

// syncedByPlatform is the games this device has synced, grouped under the
// platforms the user still has mapped.
func (s *SyncedGamesScreen) syncedByPlatform(input SyncedGamesInput) []catalog.PlatformGroup {
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil
	}

	romIDs := manager.GetSyncedRomIDs(input.DeviceID)
	if len(romIDs) == 0 {
		return nil
	}

	games, _ := manager.GetGamesByIDs(romIDs)
	if len(games) == 0 {
		return nil
	}

	var platforms []romm.Platform
	if input.Platforms != nil {
		platforms = *input.Platforms
	}
	return catalog.GroupByPlatform(games, platforms)
}

func (s *SyncedGamesScreen) showGames(client *romm.Client, config *settings.Config, platformName string, games []romm.Rom) *slotChange {
	items := make([]gaba.MenuItem, len(games))
	for i, game := range games {
		items[i] = gaba.MenuItem{Text: game.Name, Metadata: game.ID}
	}

	selected, visibleStart := 0, 0

	for {
		options := gaba.DefaultListOptions(platformName, items)
		options.FooterHelpItems = []gaba.FooterHelpItem{FooterBack(), FooterSelect()}
		options.SelectedIndex = selected
		options.VisibleStartIndex = visibleStart
		options.StatusBar = StatusBar()

		result, err := gaba.List(options)
		if err != nil || result.Action != gaba.ListActionSelected {
			return nil
		}

		selected = result.Selected[0]
		visibleStart = max(0, selected-result.VisiblePosition)

		romID, _ := items[selected].Metadata.(int)
		if change := s.showGame(client, config, romID, items[selected].Text); change != nil {
			return change
		}
	}
}

func (s *SyncedGamesScreen) showGame(client *romm.Client, config *settings.Config, romID int, gameName string) *slotChange {
	summary, err := s.loadSummary(client, romID)
	if err != nil {
		gaba.ConfirmationMessage(
			localize("synced_games_detail_error", "Failed to load save details."),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return nil
	}

	before := config.GetSlotPreference(romID)

	for {
		options := gaba.DefaultInfoScreenOptions()
		options.Sections = saveSummarySections(config, romID, summary)
		options.ShowThemeBackground = false
		options.ShowScrollbar = true
		options.ConfirmButton = buttons.VirtualButtonUnassigned
		options.ActionButton = buttons.VirtualButtonY
		options.AllowAction = true

		footer := []gaba.FooterHelpItem{
			FooterBack(),
			{ButtonName: "Y", HelpText: localize("game_options_save_slot", "Save Slot")},
		}

		result, err := gaba.DetailScreen(gameName, options, footer)
		if err != nil || result.Action != gaba.DetailActionTriggered {
			return nil
		}

		if !s.pickSlot(config, romID, summary) {
			continue
		}

		chosen := config.GetSlotPreference(romID)
		if chosen == before {
			continue
		}

		// A slot the server has never heard of holds nothing to sync down, so
		// the local saves are uploaded into it instead.
		change := &slotChange{RomID: romID}
		if !saves.HasSlot(summary, chosen) {
			change.SlotName = chosen
		}
		return change
	}
}

func (s *SyncedGamesScreen) loadSummary(client *romm.Client, romID int) (romm.SaveSummary, error) {
	var summary romm.SaveSummary
	var err error

	gaba.ProcessMessage(
		localize("synced_games_loading_detail", "Loading save details..."),
		gaba.ProcessMessageOptions{ShowThemeBackground: true},
		func() (any, error) {
			summary, err = client.GetSaveSummary(romID)
			return nil, err
		},
	)

	return summary, err
}

// pickSlot asks which slot to sync, reporting whether the choice changed
// anything worth acting on.
func (s *SyncedGamesScreen) pickSlot(config *settings.Config, romID int, summary romm.SaveSummary) bool {
	label := localize("game_options_save_slot", "Save Slot")
	slots := BuildSlotOptions(config, romID, saves.SlotNames(summary))

	result, err := gaba.OptionsList(
		label,
		gaba.OptionListSettings{
			FooterHelpItems: OptionsListFooter(),
			StatusBar:       StatusBar(),
			UseSmallTitle:   true,
		},
		[]gaba.ItemWithOptions{{
			Item:           gaba.MenuItem{Text: label},
			Options:        slots.Options,
			SelectedOption: slots.SelectedIdx,
		}},
	)
	if err != nil || len(result.Items) == 0 {
		return false
	}

	row := result.Items[0]
	if row.SelectedOption < 0 || row.SelectedOption >= len(row.Options) {
		return false
	}

	// The "New Slot..." entry carries an empty value until the user types one.
	chosen, ok := row.Options[row.SelectedOption].Value.(string)
	if !ok || chosen == "" {
		return false
	}

	config.SetSlotPreference(romID, chosen)
	if err := settings.SaveSlotPreferences(config); err != nil {
		gaba.GetLogger().Warn("Failed to save slot preferences", "error", err)
	}
	return true
}

func saveSummarySections(config *settings.Config, romID int, summary romm.SaveSummary) []gaba.Section {
	sections := []gaba.Section{
		gaba.NewInfoSection(
			localize("synced_games_overview", "Overview"),
			[]gaba.MetadataItem{
				{
					Label: localize("synced_games_total_saves", "Total Saves"),
					Value: fmt.Sprintf("%d", summary.TotalCount),
				},
				{
					Label: localize("synced_games_active_slot", "Active Slot"),
					Value: config.GetSlotPreference(romID),
				},
			},
		),
	}

	for _, slot := range summary.Slots {
		sections = append(sections, gaba.NewInfoSection(saves.SlotName(slot.Slot), []gaba.MetadataItem{
			{
				Label: localize("synced_games_save_count", "Save Count"),
				Value: fmt.Sprintf("%d", slot.Count),
			},
			{
				Label: localize("synced_games_latest_save", "Latest Save"),
				Value: slot.Latest.UpdatedAt.Format("Jan 2, 2006 3:04 PM"),
			},
			{
				Label: localize("synced_games_latest_file", "File"),
				Value: slot.Latest.FileName,
			},
		}))
	}

	return sections
}

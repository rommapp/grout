package ui

import (
	"grout/cache"
	"sort"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
	buttons "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/constants"
	"github.com/BrandonKowalski/gabagool/v2/pkg/gabagool/i18n"
	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
)

type SyncHistoryInput struct {
	DeviceID string
}

type SyncHistoryOutput struct {
	Action SyncHistoryAction
}

type SyncHistoryScreen struct{}

func NewSyncHistoryScreen() *SyncHistoryScreen {
	return &SyncHistoryScreen{}
}

func (s *SyncHistoryScreen) Draw(input SyncHistoryInput) (SyncHistoryOutput, error) {
	output := SyncHistoryOutput{Action: SyncHistoryActionBack}

	cm := cache.GetCacheManager()
	if cm == nil {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "sync_history_no_cache", Other: "Cache not available."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	records := cm.GetSaveSyncHistory(input.DeviceID, 50)

	if len(records) == 0 {
		gaba.ConfirmationMessage(
			i18n.Localize(&goi18n.Message{ID: "sync_history_empty", Other: "No sync history found."}, nil),
			ContinueFooter(),
			gaba.MessageOptions{},
		)
		return output, nil
	}

	sections := s.buildSections(records, cm)

	options := gaba.DefaultInfoScreenOptions()
	options.Sections = sections
	options.ShowThemeBackground = false
	options.ShowScrollbar = true
	options.ConfirmButton = buttons.VirtualButtonUnassigned

	gaba.DetailScreen(
		i18n.Localize(&goi18n.Message{ID: "sync_history_title", Other: "Sync History"}, nil),
		options,
		[]gaba.FooterHelpItem{FooterBack()},
	)

	return output, nil
}

func actionIcon(action string) string {
	switch action {
	case "upload":
		return buttons.CloudUpload
	case "download":
		return buttons.CloudDownload
	default:
		return action
	}
}

func actionSortKey(action string) int {
	switch action {
	case "download":
		return 0
	case "upload":
		return 1
	default:
		return 2
	}
}

func (s *SyncHistoryScreen) buildSections(records []cache.SaveSyncRecord, cm *cache.Manager) []gaba.Section {
	// Build ROM ID → platform slug lookup
	romIDs := make([]int, 0, len(records))
	seen := make(map[int]bool)
	for _, r := range records {
		if !seen[r.RomID] {
			seen[r.RomID] = true
			romIDs = append(romIDs, r.RomID)
		}
	}

	platformByRomID := make(map[int]string)
	if games, err := cm.GetGamesByIDs(romIDs); err == nil {
		for _, g := range games {
			platformByRomID[g.ID] = g.PlatformFSSlug
		}
	}

	type rowEntry struct {
		action   string
		romName  string
		platform string
		time     string
	}

	type dateGroup struct {
		date    string
		entries []rowEntry
	}

	groups := make([]dateGroup, 0)
	groupIndex := make(map[string]int)

	for _, r := range records {
		dateKey := r.SyncedAt.Format("January 2, 2006")
		timeStr := r.SyncedAt.Format("15:04")
		platform := platformByRomID[r.RomID]

		entry := rowEntry{
			action:   r.Action,
			romName:  r.RomName,
			platform: platform,
			time:     timeStr,
		}

		idx, ok := groupIndex[dateKey]
		if !ok {
			idx = len(groups)
			groupIndex[dateKey] = idx
			groups = append(groups, dateGroup{date: dateKey})
		}
		groups[idx].entries = append(groups[idx].entries, entry)
	}

	headers := []string{
		"",
		i18n.Localize(&goi18n.Message{ID: "sync_history_col_game", Other: "Game"}, nil),
		i18n.Localize(&goi18n.Message{ID: "sync_history_col_platform", Other: "Platform"}, nil),
		i18n.Localize(&goi18n.Message{ID: "sync_history_col_time", Other: "Time"}, nil),
	}

	var sections []gaba.Section
	for _, g := range groups {
		// Sort by action group (downloads first, then uploads), then alphabetically by name
		sort.Slice(g.entries, func(i, j int) bool {
			ai, aj := actionSortKey(g.entries[i].action), actionSortKey(g.entries[j].action)
			if ai != aj {
				return ai < aj
			}
			return g.entries[i].romName < g.entries[j].romName
		})

		rows := make([]gaba.TableRow, len(g.entries))
		for i, e := range g.entries {
			rows[i] = gaba.TableRow{Cells: []string{actionIcon(e.action), e.romName, e.platform, e.time}}
		}

		sections = append(sections, gaba.NewTableSection(g.date, headers, rows, gaba.TableGridRowDividers))
	}

	return sections
}

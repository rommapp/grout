package saves

import (
	"slices"
	"strings"
	"time"

	"grout/cache"
)

// SyncEvent is one save that moved between the device and the server.
type SyncEvent struct {
	// Action is what happened to it: an upload or a download.
	Action  string
	RomName string
	// Platform is the game's platform as the user knows it, falling back to
	// its slug when the cache never learned a name for it.
	Platform string
	At       time.Time
}

// SyncDay gathers the events of one local day.
type SyncDay struct {
	// Date is midnight local on that day. The screen decides how to write it.
	Date   time.Time
	Events []SyncEvent
}

// SyncHistory is what a device has synced, newest day first and newest event
// first within each day.
//
// Days are local, not the UTC the records are stored in: a sync at one in the
// morning belongs to the night the user remembers, not to the previous day.
func SyncHistory(deviceID string) []SyncDay {
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil
	}

	records := manager.GetSaveSyncHistory(deviceID)
	if len(records) == 0 {
		return nil
	}

	return groupByDay(records, platformsOf(manager, records))
}

// groupByDay gathers records under the local day they happened on.
func groupByDay(records []cache.SaveSyncRecord, platforms map[int]string) []SyncDay {
	var days []SyncDay
	index := make(map[time.Time]int)

	for _, record := range records {
		at := record.SyncedAt.Local()
		date := time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, at.Location())

		position, seen := index[date]
		if !seen {
			position = len(days)
			index[date] = position
			days = append(days, SyncDay{Date: date})
		}

		days[position].Events = append(days[position].Events, SyncEvent{
			Action:   record.Action,
			RomName:  record.RomName,
			Platform: platforms[record.RomID],
			At:       at,
		})
	}

	slices.SortFunc(days, func(a, b SyncDay) int { return b.Date.Compare(a.Date) })
	for i := range days {
		slices.SortFunc(days[i].Events, func(a, b SyncEvent) int {
			if !a.At.Equal(b.At) {
				return b.At.Compare(a.At)
			}
			return strings.Compare(strings.ToLower(a.RomName), strings.ToLower(b.RomName))
		})
	}

	return days
}

// platformsOf names the platform of every game in the history, in one lookup
// rather than one per row.
func platformsOf(manager *cache.Manager, records []cache.SaveSyncRecord) map[int]string {
	romIDs := make([]int, 0, len(records))
	seen := make(map[int]bool, len(records))
	for _, record := range records {
		if !seen[record.RomID] {
			seen[record.RomID] = true
			romIDs = append(romIDs, record.RomID)
		}
	}

	games, err := manager.GetGamesByIDs(romIDs)
	if err != nil {
		return nil
	}

	names := make(map[int]string, len(games))
	for _, game := range games {
		name := game.PlatformDisplayName
		if name == "" {
			name = game.PlatformFSSlug
		}
		names[game.ID] = name
	}
	return names
}

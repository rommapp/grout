package saves

import (
	"encoding/json"
	"os"
	"time"

	"grout/files"
)

// RecentSyncWindow is how long a game counts as just synced. Launching it again
// inside this time skips the sync that would otherwise run first.
const RecentSyncWindow = 10 * time.Minute

// recentSyncsFile sits beside the config, like the other small bits of state
// Grout keeps for itself.
const recentSyncsFile = "recent_syncs.json"

// recentSyncsKept bounds how long a record is remembered, far longer than it
// can matter, so the file does not grow with every game ever launched.
const recentSyncsKept = 24 * time.Hour

type recentSyncData struct {
	// All is when a sync of every game last finished.
	All int64 `json:"all,omitempty"`
	// Roms is when each game was last synced by itself, by ROM path.
	Roms map[string]int64 `json:"roms,omitempty"`
}

// RecentSyncs remembers when games were last synced, so a launch right after a
// sync does not make the player wait for another.
type RecentSyncs struct {
	path string
	data recentSyncData
}

// LoadRecentSyncs reads the record. One that is missing or unreadable is the
// same as one that is empty: the worst that happens is one more sync.
func LoadRecentSyncs() *RecentSyncs { return loadRecentSyncsFrom(recentSyncsFile) }

func loadRecentSyncsFrom(path string) *RecentSyncs {
	recent := &RecentSyncs{path: path, data: recentSyncData{Roms: map[string]int64{}}}
	if raw, err := os.ReadFile(path); err == nil {
		var data recentSyncData
		if json.Unmarshal(raw, &data) == nil {
			recent.data = data
		}
	}
	if recent.data.Roms == nil {
		recent.data.Roms = map[string]int64{}
	}
	return recent
}

// SyncedRecently reports whether the game was synced inside the window, and how
// long ago. A record from the future, which a device whose clock was set back
// would leave, is not trusted.
func (r *RecentSyncs) SyncedRecently(romPath string, now time.Time) (time.Duration, bool) {
	last := r.data.All
	if stamp := r.data.Roms[canonicalPath(romPath)]; stamp > last {
		last = stamp
	}
	if last == 0 {
		return 0, false
	}
	age := now.Sub(time.Unix(last, 0))
	return age, age >= 0 && age < RecentSyncWindow
}

// RecordRom notes that one game has just been synced.
func (r *RecentSyncs) RecordRom(romPath string, now time.Time) {
	r.data.Roms[canonicalPath(romPath)] = now.Unix()
}

// RecordAll notes that every game has just been synced.
func (r *RecentSyncs) RecordAll(now time.Time) { r.data.All = now.Unix() }

// Save writes the record, dropping what is too old to matter.
func (r *RecentSyncs) Save(now time.Time) error {
	for rom, stamp := range r.data.Roms {
		if now.Sub(time.Unix(stamp, 0)) > recentSyncsKept {
			delete(r.data.Roms, rom)
		}
	}
	if r.data.All != 0 && now.Sub(time.Unix(r.data.All, 0)) > recentSyncsKept {
		r.data.All = 0
	}

	raw, err := json.Marshal(r.data)
	if err != nil {
		return err
	}
	return files.WriteFileAtomic(r.path, raw, 0644)
}

// RecordFullSync notes, on disk, that every game has just been synced.
func RecordFullSync(now time.Time) error {
	recent := LoadRecentSyncs()
	recent.RecordAll(now)
	return recent.Save(now)
}

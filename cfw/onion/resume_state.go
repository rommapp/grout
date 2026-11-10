package onion

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// keptSetAsideStates is how many set-aside copies of one resume state are kept.
const keptSetAsideStates = 3

// StatesDirectory is where RetroArch keeps save states, in a folder per core.
func StatesDirectory() string {
	return filepath.Join(GetBasePath(), "Saves", "CurrentProfile", "states")
}

// SetAsideResumeState renames a game's auto save state, and its preview
// picture, so the game starts from its save file instead of resuming. It
// returns how many files it renamed.
//
// Onion resumes a game from <rom>.state.auto. That state is a snapshot of the
// game at the last quit, so when a newer save has been brought in from another
// device the state would put the old progress back on top of it. Nothing is
// deleted: the files are renamed the way Onion's own netplay scripts set them
// aside, and the last few are kept.
func SetAsideResumeState(romPath string) (int, error) {
	return setAsideResumeState(StatesDirectory(), romPath, time.Now())
}

func setAsideResumeState(statesDir, romPath string, now time.Time) (int, error) {
	name := strings.TrimSuffix(filepath.Base(romPath), filepath.Ext(romPath))
	stamp := now.Format("20060102-150405")

	cores, err := os.ReadDir(statesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	moved := 0
	var failed []string
	for _, core := range cores {
		if !core.IsDir() {
			continue
		}
		dir := filepath.Join(statesDir, core.Name())
		for _, suffix := range []string{".state.auto", ".state.auto.png"} {
			path := filepath.Join(dir, name+suffix)
			if _, err := os.Stat(path); err != nil {
				continue
			}
			if err := os.Rename(path, path+"_"+stamp); err != nil {
				failed = append(failed, err.Error())
				continue
			}
			moved++
		}
		pruneSetAside(dir, name+".state.auto", stamp)
		pruneSetAside(dir, name+".state.auto.png", stamp)
	}

	if len(failed) > 0 {
		return moved, fmt.Errorf("could not set aside %d files: %s", len(failed), strings.Join(failed, "; "))
	}
	return moved, nil
}

// pruneSetAside removes the oldest set-aside copies of one file past the
// number kept. The names sort by time because the stamp does.
//
// The copy just made is never removed. The stamp is the time on the device, and a
// device whose clock has been set back would otherwise find its newest copy
// sorting as its oldest.
func pruneSetAside(dir, base, fresh string) {
	matches, err := filepath.Glob(filepath.Join(dir, escapeGlob(base)+"_*"))
	if err != nil || len(matches) <= keptSetAsideStates {
		return
	}
	sort.Strings(matches)
	excess := len(matches) - keptSetAsideStates
	for _, old := range matches {
		if excess == 0 {
			break
		}
		if strings.HasSuffix(old, "_"+fresh) {
			continue
		}
		if os.Remove(old) == nil {
			excess--
		}
	}
}

func escapeGlob(s string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`).Replace(s)
}

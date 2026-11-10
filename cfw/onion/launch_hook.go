package onion

import (
	"bytes"
	"fmt"
	"grout/files"
	"os"
	"path/filepath"
	"strings"
)

// launchHookMarker tags the line Grout adds to an emulator's launch script, so
// it can be found again and taken out without disturbing anything else in the
// file.
const launchHookMarker = "# grout-sync-on-launch"

// SyncScriptPath is the script Grout ships that the launch hook calls.
func SyncScriptPath() string {
	return filepath.Join(GetBasePath(), "App", "Grout", "sync_on_launch.sh")
}

// SetLaunchHook makes every emulator on the device run Grout's sync script
// before it starts a game (before) and after the game is quit (after), and
// takes the calls back out of every launch script when neither is wanted. It
// returns how many launch scripts it changed.
//
// Onion has no hook of its own for this. Each emulator package carries a
// launch.sh that receives the ROM path as its first argument and runs the
// emulator in the foreground, so a call at the top runs before the game and
// one at the bottom runs once it has been quit. Onion updates replace those
// files; setting the hook again puts it back, and a launch script without the
// lines simply launches as before.
func SetLaunchHook(before, after bool) (int, error) {
	return setLaunchHook(filepath.Join(GetBasePath(), "Emu"), SyncScriptPath(), before, after)
}

// InstallSyncSaveMenuEntry adds a "Sync save" entry to the game list's Y menu.
// It does not depend on the launch hook: the entry only ever syncs when the
// player picks it.
func InstallSyncSaveMenuEntry() error {
	return setSyncSaveMenuEntry(SyncSaveMenuEntryPath(), SyncScriptPath(), true)
}

// SyncSaveMenuEntryPath is where the Y menu's "Sync save" script goes. Onion
// lists every script in this folder in the game list's options menu.
func SyncSaveMenuEntryPath() string {
	return filepath.Join(GetBasePath(), "App", "romscripts", "Grout - Sync save.sh")
}

// syncSaveMenuEntry is the script Onion lists. Onion reads scriptlabel and
// scriptinfo straight out of the file, and runs it with the ROM path as its
// first argument. Whatever it exits with is passed back to the game list, where
// zero means "launch the highlighted game", so it always exits 1.
func syncSaveMenuEntry(script string) string {
	return "#!/bin/sh\n" +
		"scriptlabel=\"Sync save (Grout)\"\n" +
		"scriptinfo=\"Syncs the save for this game\\nwith RomM right now\"\n" +
		launchHookMarker + "\n" +
		fmt.Sprintf("if [ -f \"%s\" ]; then sh \"%s\" --now \"$1\"; fi\n", script, script) +
		"exit 1\n"
}

// setSyncSaveMenuEntry adds or removes the Y menu script. It only ever removes
// a file it wrote.
func setSyncSaveMenuEntry(path, script string, enabled bool) error {
	if !enabled {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !strings.Contains(string(data), launchHookMarker) {
			return nil
		}
		return os.Remove(path)
	}

	if existing, err := os.ReadFile(path); err == nil && !strings.Contains(string(existing), launchHookMarker) {
		return fmt.Errorf("%s exists and is not Grout's", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return files.WriteFileAtomic(path, []byte(syncSaveMenuEntry(script)), 0755)
}

func launchScripts(emuDir string) ([]string, error) {
	return filepath.Glob(filepath.Join(emuDir, "*", "launch.sh"))
}

// Run through sh rather than as a program: the card may not keep the execute
// bit on a file copied in from a zip.
func hookCall(script, args string) string {
	return fmt.Sprintf(`if [ -f "%s" ]; then sh "%s" %s; fi %s`, script, script, args, launchHookMarker)
}

func beforeLine(script string) string { return hookCall(script, `"$1"`) }

// afterLines run once the emulator has exited. The emulator's exit code is
// what the frontend reads to decide whether the game crashed, so it is held
// across the sync and handed back.
func afterLines(script string) []string {
	return []string{
		"grout_exit=$? " + launchHookMarker,
		hookCall(script, `--after "$1"`),
		`exit "$grout_exit" ` + launchHookMarker,
	}
}

func setLaunchHook(emuDir, script string, before, after bool) (int, error) {
	paths, err := launchScripts(emuDir)
	if err != nil {
		return 0, err
	}

	changed := 0
	var failed []string
	for _, path := range paths {
		done, err := editLaunchScript(path, func(lines []string) []string {
			lines = withoutHook(lines)
			if before {
				// After the shebang, so it runs before the script changes
				// directory or starts anything.
				at := 0
				if len(lines) > 0 && strings.HasPrefix(lines[0], "#!") {
					at = 1
				}
				lines = append(lines[:at], append([]string{beforeLine(script)}, lines[at:]...)...)
			}
			if after {
				lines = append(lines, afterLines(script)...)
			}
			return lines
		})
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		if done {
			changed++
		}
	}

	if len(failed) > 0 {
		return changed, fmt.Errorf("could not update %d launch scripts: %s", len(failed), strings.Join(failed, "; "))
	}
	return changed, nil
}

func withoutHook(lines []string) []string {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, launchHookMarker) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// editLaunchScript rewrites a launch script line by line, keeping its line
// endings and permissions, and reports whether the content changed. A file
// that comes out the same is not written.
func editLaunchScript(path string, edit func([]string) []string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	newline := "\n"
	if bytes.Contains(original, []byte("\r\n")) {
		newline = "\r\n"
	}
	text := strings.ReplaceAll(string(original), "\r\n", "\n")
	trailing := strings.HasSuffix(text, "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")

	updated := strings.Join(edit(lines), newline)
	if trailing {
		updated += newline
	}
	if updated == string(original) {
		return false, nil
	}

	return true, files.WriteFileAtomic(path, []byte(updated), info.Mode().Perm())
}

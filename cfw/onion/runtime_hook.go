package onion

import (
	"bytes"
	"fmt"
	"grout/files"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runtimeHookFlag exists while Onion's own runtime is calling Grout's sync, so
// the call in an emulator's launch.sh does not sync the same game a second time.
const runtimeHookFlag = "/tmp/.grout_runtime_hook"

// The two lines of Onion's runtime.sh the hook goes around. It runs the command
// for every game, whichever way that command was built.
const (
	runtimeLaunchLine = `TZ="$TZ_VALUE" $sysdir/cmd_to_run.sh`
	runtimeStatusLine = `retval=$?`
)

// RuntimePath is Onion's runtime, the script that runs the frontend, launches
// every game, and returns to the frontend afterwards.
func RuntimePath() string {
	return filepath.Join(GetBasePath(), ".tmp_update", "runtime.sh")
}

// SetRuntimeHook makes Onion's runtime call Grout's sync around every game, or
// takes the calls back out.
//
// An emulator's launch.sh is skipped when a game has its own core (Y menu >
// Game core): Onion then builds a RetroArch command itself and runs that
// instead, so the calls Grout puts in launch.sh never happen. The runtime is
// the one place every launch passes through.
//
// It edits a script Grout does not own, so it is careful: it refuses unless
// the two lines it goes around are found exactly once, it checks the result
// still parses, it keeps a copy of the original beside it, and it writes by
// renaming a new file into place, which the running runtime does not notice
// until Onion next starts. Onion updates replace the file; setting the hook
// again puts it back.
func SetRuntimeHook(enabled bool) error {
	return setRuntimeHook(RuntimePath(), SyncScriptPath(), enabled, parses)
}

func setRuntimeHook(path, script string, enabled bool, validate func(string) error) error {
	if enabled {
		// Once, so it is the file as Onion shipped it and not a patched copy.
		if err := backUpOnce(path); err != nil {
			return fmt.Errorf("could not keep a copy of %s: %w", filepath.Base(path), err)
		}
	}

	_, err := editScript(path, func(lines []string) ([]string, error) {
		lines = withoutHook(lines)
		if !enabled {
			return lines, nil
		}
		return aroundLaunch(lines, script)
	}, validate)
	return err
}

// aroundLaunch puts the before call above the line that runs the game, and the
// after call under the line that keeps its exit code.
func aroundLaunch(lines []string, script string) ([]string, error) {
	at := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != runtimeLaunchLine {
			continue
		}
		if at != -1 {
			return nil, fmt.Errorf("the line that runs the game appears more than once")
		}
		at = i
	}
	if at == -1 {
		return nil, fmt.Errorf("the line that runs the game was not found; this Onion version is not supported")
	}
	if at+1 >= len(lines) || strings.TrimSpace(lines[at+1]) != runtimeStatusLine {
		return nil, fmt.Errorf("the line after the one that runs the game is not %q; this Onion version is not supported", runtimeStatusLine)
	}

	indent := lines[at][:len(lines[at])-len(strings.TrimLeft(lines[at], " \t"))]
	before := indent + fmt.Sprintf(`if [ $is_game -eq 1 ] && [ -f "%s" ]; then touch %s; sh "%s" --from-runtime "$rompath"; fi %s`,
		script, runtimeHookFlag, script, launchHookMarker)
	after := []string{
		indent + fmt.Sprintf(`if [ $is_game -eq 1 ] && [ -f "%s" ]; then sh "%s" --from-runtime --after "$rompath"; fi %s`,
			script, script, launchHookMarker),
		indent + "rm -f " + runtimeHookFlag + " " + launchHookMarker,
	}

	out := make([]string, 0, len(lines)+3)
	out = append(out, lines[:at]...)
	out = append(out, before, lines[at], lines[at+1])
	out = append(out, after...)
	out = append(out, lines[at+2:]...)
	return out, nil
}

// backUpOnce copies a file next to itself, unless that has been done.
func backUpOnce(path string) error {
	backup := path + ".grout-backup"
	if _, err := os.Stat(backup); err == nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// A file that already carries the hook is not a copy of the original.
	if bytes.Contains(data, []byte(launchHookMarker)) {
		return nil
	}
	return files.WriteFileAtomic(backup, data, 0644)
}

// parses reports whether the shell accepts the script, without running it.
func parses(content string) error {
	tmp, err := os.CreateTemp("", "grout-parse-*.sh")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if out, err := exec.Command("sh", "-n", tmp.Name()).CombinedOutput(); err != nil {
		return fmt.Errorf("the edited script does not parse: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

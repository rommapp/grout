package onion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The part of Onion's runtime.sh that runs a game, trimmed to what the hook
// cares about.
const runtimeFixture = `#!/bin/sh
sysdir=/mnt/SDCARD/.tmp_update

launch_game() {
    is_game=1
    rompath="/mnt/SDCARD/Roms/GBA/Game.gba"

    if [ $is_game -eq 1 ]; then
        # GAME LAUNCH
        cd /mnt/SDCARD/RetroArch

        # make the cmd_to_run shell env aware of the new timezone
        TZ="$TZ_VALUE" $sysdir/cmd_to_run.sh
        retval=$?

        if [ -f /tmp/new_res_available ]; then
            echo restore
        fi
    fi
    echo "cmd retval: $retval"
}
launch_game
`

func writeRuntime(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRuntimeHookGoesAroundTheLineThatRunsTheGame(t *testing.T) {
	path := writeRuntime(t, runtimeFixture)

	if err := setRuntimeHook(path, "/mnt/SDCARD/App/Grout/sync_on_launch.sh", true, parses); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(read(t, path), "\n")
	at := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == runtimeLaunchLine {
			at = i
		}
	}
	if at < 1 {
		t.Fatal("the launch line is gone")
	}
	if !strings.Contains(lines[at-1], `--from-runtime "$rompath"`) || !strings.Contains(lines[at-1], "touch "+runtimeHookFlag) {
		t.Errorf("before call = %q", lines[at-1])
	}
	if strings.TrimSpace(lines[at+1]) != runtimeStatusLine {
		t.Errorf("the exit code is no longer captured straight after the game: %q", lines[at+1])
	}
	if !strings.Contains(lines[at+2], `--from-runtime --after "$rompath"`) {
		t.Errorf("after call = %q", lines[at+2])
	}
	if !strings.Contains(lines[at+3], "rm -f "+runtimeHookFlag) {
		t.Errorf("flag not cleared: %q", lines[at+3])
	}
	if !strings.HasPrefix(lines[at-1], "        ") {
		t.Errorf("indentation lost: %q", lines[at-1])
	}
	if _, err := os.Stat(path + ".grout-backup"); err != nil {
		t.Error("no copy of the original was kept")
	}
	if read(t, path+".grout-backup") != runtimeFixture {
		t.Error("the copy is not the original")
	}
}

func TestRuntimeHookTurnsOffToExactlyTheOriginal(t *testing.T) {
	path := writeRuntime(t, runtimeFixture)

	setRuntimeHook(path, "/s.sh", true, parses)
	if err := setRuntimeHook(path, "/s.sh", false, parses); err != nil {
		t.Fatal(err)
	}
	if got := read(t, path); got != runtimeFixture {
		t.Errorf("not restored:\n%s", got)
	}
}

func TestRuntimeHookIsAddedOnlyOnce(t *testing.T) {
	path := writeRuntime(t, runtimeFixture)

	setRuntimeHook(path, "/s.sh", true, parses)
	once := read(t, path)
	setRuntimeHook(path, "/s.sh", true, parses)

	if read(t, path) != once {
		t.Error("a second install changed the file")
	}
}

func TestRuntimeHookRefusesAnOnionItDoesNotRecognise(t *testing.T) {
	cases := map[string]string{
		"launch line missing": strings.ReplaceAll(runtimeFixture, runtimeLaunchLine, `$sysdir/cmd_to_run.sh`),
		"status line moved":   strings.ReplaceAll(runtimeFixture, "        retval=$?\n", "        sync\n        retval=$?\n"),
		"launch line twice":   strings.Replace(runtimeFixture, "launch_game\n", "        "+runtimeLaunchLine+"\n        retval=$?\nlaunch_game\n", 1),
	}
	for name, content := range cases {
		path := writeRuntime(t, content)
		if err := setRuntimeHook(path, "/s.sh", true, parses); err == nil {
			t.Errorf("%s: patched anyway", name)
		}
		if read(t, path) != content {
			t.Errorf("%s: the file was changed", name)
		}
	}
}

func TestRuntimeHookNeverWritesAScriptThatDoesNotParse(t *testing.T) {
	path := writeRuntime(t, runtimeFixture)
	broken := func(string) error { return os.ErrInvalid }

	if err := setRuntimeHook(path, "/s.sh", true, broken); err == nil {
		t.Error("wrote a script the check rejected")
	}
	if read(t, path) != runtimeFixture {
		t.Error("the file was changed")
	}
}

func TestPatchedRuntimeStillParsesAndKeepsItsMode(t *testing.T) {
	path := writeRuntime(t, runtimeFixture)
	setRuntimeHook(path, "/s.sh", true, parses)

	if err := parses(read(t, path)); err != nil {
		t.Error(err)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0755 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
}

package onion

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const gbLaunch = "#!/bin/sh\necho $0 $*\ncd /mnt/SDCARD/RetroArch/\n./retroarch \"$1\"\n"

func writeEmu(t *testing.T, emuDir, name, content string, mode os.FileMode) string {
	t.Helper()
	dir := filepath.Join(emuDir, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "launch.sh")
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBeforePutsTheHookRightAfterTheShebang(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	changed, err := setLaunchHook(emu, "/mnt/SDCARD/App/Grout/sync_on_launch.sh", true, false)
	if err != nil || changed != 1 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}

	got := read(t, path)
	lines := strings.Split(got, "\n")
	if lines[0] != "#!/bin/sh" {
		t.Errorf("shebang moved: %q", lines[0])
	}
	if !strings.Contains(lines[1], launchHookMarker) || !strings.Contains(lines[1], `sh "/mnt/SDCARD/App/Grout/sync_on_launch.sh" "$1"`) {
		t.Errorf("hook not on line 2: %q", lines[1])
	}
	if lines[2] != "echo $0 $*" {
		t.Errorf("original body disturbed: %q", lines[2])
	}
	if strings.Contains(got, "--after") {
		t.Error("after-quit call installed without being asked for")
	}
}

func TestAfterGoesAtTheEnd(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	if _, err := setLaunchHook(emu, "/s.sh", true, true); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSuffix(read(t, path), "\n"), "\n")
	n := len(lines)
	if lines[n-4] != `./retroarch "$1"` {
		t.Errorf("emulator command is no longer last before the hook: %q", lines[n-4])
	}
	if !strings.HasPrefix(lines[n-3], "grout_exit=$?") {
		t.Errorf("exit code not captured straight after the emulator: %q", lines[n-3])
	}
	if !strings.Contains(lines[n-2], `--after "$1"`) {
		t.Errorf("after call = %q", lines[n-2])
	}
	if !strings.HasPrefix(lines[n-1], `exit "$grout_exit"`) {
		t.Errorf("exit code not handed back: %q", lines[n-1])
	}
}

func TestChangingTheModeAddsAndDropsOnlyWhatChanged(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	setLaunchHook(emu, "/s.sh", true, true)
	changed, err := setLaunchHook(emu, "/s.sh", true, false)
	if err != nil || changed != 1 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	got := read(t, path)
	if strings.Contains(got, "--after") || strings.Contains(got, "grout_exit") {
		t.Errorf("after-quit lines left behind:\n%s", got)
	}
	if n := strings.Count(got, launchHookMarker); n != 1 {
		t.Errorf("%d marked lines, want 1", n)
	}
}

func TestSettingTheSameModeTwiceChangesNothing(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	setLaunchHook(emu, "/s.sh", true, true)
	once := read(t, path)

	changed, err := setLaunchHook(emu, "/s.sh", true, true)
	if err != nil || changed != 0 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	if read(t, path) != once {
		t.Error("second install rewrote the file")
	}
}

func TestTurningItOffRestoresTheOriginalExactly(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	setLaunchHook(emu, "/s.sh", true, true)
	changed, err := setLaunchHook(emu, "/s.sh", false, false)
	if err != nil || changed != 1 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	if got := read(t, path); got != gbLaunch {
		t.Errorf("not restored:\n%q", got)
	}
}

func TestHookKeepsPermissionsAndLineEndings(t *testing.T) {
	emu := t.TempDir()
	crlf := strings.ReplaceAll(gbLaunch, "\n", "\r\n")
	path := writeEmu(t, emu, "GBA", crlf, 0755)

	setLaunchHook(emu, "/s.sh", true, true)

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	if strings.Contains(strings.ReplaceAll(read(t, path), "\r\n", ""), "\n") {
		t.Error("mixed line endings")
	}

	setLaunchHook(emu, "/s.sh", false, false)
	if read(t, path) != crlf {
		t.Error("CRLF file not restored")
	}
}

func TestScriptWithoutShebangGetsTheHookOnTop(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "X", "echo hi\n", 0755)

	setLaunchHook(emu, "/s.sh", true, false)

	if first := strings.SplitN(read(t, path), "\n", 2)[0]; !strings.Contains(first, launchHookMarker) {
		t.Errorf("first line = %q", first)
	}
}

// The rewritten script is run for real: the sync script is a stub that records
// how it was called, and the "emulator" exits with a chosen code.
func TestPatchedScriptCallsTheSyncAroundTheGameAndKeepsItsExitCode(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	stub := filepath.Join(dir, "sync_on_launch.sh")
	stubBody := "echo \"$@\" >> " + log + "\n"
	if err := os.WriteFile(stub, []byte(stubBody), 0644); err != nil {
		t.Fatal(err)
	}

	emu := filepath.Join(dir, "Emu")
	// The real scripts end on the emulator's command rather than an exit, so
	// the game's status is the status of its last command.
	game := "#!/bin/sh\necho playing >> " + log + "\n(exit 3)\n"
	path := writeEmu(t, emu, "GB", game, 0755)

	if _, err := setLaunchHook(emu, stub, true, true); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("sh", path, "/roms/Tetris.gb")
	err := cmd.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("exit = %v, want the game's 3", err)
	}

	want := "/roms/Tetris.gb\nplaying\n--after /roms/Tetris.gb\n"
	if got := read(t, log); got != want {
		t.Errorf("calls:\n%q\nwant:\n%q", got, want)
	}
}

func TestMissingSyncScriptLetsTheGameStartAsBefore(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	emu := filepath.Join(dir, "Emu")
	path := writeEmu(t, emu, "GB", "#!/bin/sh\necho playing >> "+log+"\n", 0755)

	setLaunchHook(emu, filepath.Join(dir, "not-installed.sh"), true, true)

	if err := exec.Command("sh", path, "/roms/a.gb").Run(); err != nil {
		t.Fatalf("launch failed: %v", err)
	}
	if got := read(t, log); got != "playing\n" {
		t.Errorf("calls = %q", got)
	}
}

func TestMenuEntryIsWhatOnionExpectsOfAScript(t *testing.T) {
	entry := syncSaveMenuEntry("/mnt/SDCARD/App/Grout/sync_on_launch.sh")

	for _, want := range []string{
		"#!/bin/sh\n",
		`scriptlabel="Sync save (Grout)"`,
		`scriptinfo="`,
		`sh "/mnt/SDCARD/App/Grout/sync_on_launch.sh" --now "$1"`,
	} {
		if !strings.Contains(entry, want) {
			t.Errorf("entry is missing %q:\n%s", want, entry)
		}
	}
	// Exiting 0 would make the game list launch the highlighted game.
	if !strings.HasSuffix(entry, "exit 1\n") {
		t.Errorf("entry does not end by exiting 1:\n%s", entry)
	}
	if n := strings.Count(entry, "scriptlabel"); n != 1 {
		t.Errorf("scriptlabel appears %d times; Onion reads the first match", n)
	}
}

func TestMenuEntryComesAndGoesWithTheSetting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "App", "romscripts", "Grout - Sync save.sh")

	if err := setSyncSaveMenuEntry(path, "/s.sh", true); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("entry not installed executable: %v %v", err, info)
	}

	if err := setSyncSaveMenuEntry(path, "/s.sh", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("entry still there after turning the setting off")
	}
	if err := setSyncSaveMenuEntry(path, "/s.sh", false); err != nil {
		t.Errorf("removing what is not there failed: %v", err)
	}
}

func TestMenuEntryNeverTouchesAScriptThatIsNotGrouts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Grout - Sync save.sh")
	mine := "#!/bin/sh\necho somebody else\n"
	os.WriteFile(path, []byte(mine), 0755)

	if err := setSyncSaveMenuEntry(path, "/s.sh", true); err == nil {
		t.Error("overwrote someone else's script")
	}
	if err := setSyncSaveMenuEntry(path, "/s.sh", false); err != nil {
		t.Error(err)
	}
	if read(t, path) != mine {
		t.Error("someone else's script was changed or removed")
	}
}

// A launcher that hands over with exec never reaches lines written after it, so
// no after-quit call is added to it, though the call before the game still is.
func TestALauncherThatEndsInExecGetsNoAfterQuitCall(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "X", "#!/bin/sh\ncd /mnt/SDCARD/RetroArch\nexec ./retroarch -L core.so \"$1\"\n", 0755)

	if _, err := setLaunchHook(emu, "/s.sh", true, true); err != nil {
		t.Fatal(err)
	}

	got := read(t, path)
	if !strings.Contains(got, `sh "/s.sh" "$1"`) {
		t.Error("the call before the game is missing")
	}
	if strings.Contains(got, "--after") || strings.Contains(got, "grout_exit") {
		t.Errorf("an after-quit call was added after an exec:\n%s", got)
	}
}

// Onion's own GBA launcher has an exit inside a branch; it still returns to the
// lines after it, so it is hooked on both sides.
func TestAnExitInsideABranchDoesNotStopTheAfterQuitCall(t *testing.T) {
	emu := t.TempDir()
	launcher := "#!/bin/sh\nif [ -f x ]; then\n    if [ $? -eq 0 ]; then\n        :\n    else\n        exit 0\n    fi\nfi\ncd /mnt/SDCARD/RetroArch\n./retroarch -L core.so \"$1\"\n"
	path := writeEmu(t, emu, "GBA", launcher, 0755)

	if _, err := setLaunchHook(emu, "/s.sh", true, true); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(read(t, path), "--after") {
		t.Error("the GBA-style launcher lost its after-quit call")
	}
}

func TestATopLevelExitAlsoStopsTheAfterQuitCall(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "X", "#!/bin/sh\n./retroarch \"$1\"\nexit 0\n", 0755)

	setLaunchHook(emu, "/s.sh", true, true)

	if strings.Contains(read(t, path), "--after") {
		t.Error("an after-quit call was added after a top-level exit")
	}
}

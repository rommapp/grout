package onion

import (
	"os"
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

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallPutsTheHookRightAfterTheShebang(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	changed, err := installLaunchHook(emu, "/mnt/SDCARD/App/Grout/sync_on_launch.sh")
	if err != nil || changed != 1 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}

	lines := strings.Split(read(t, path), "\n")
	if lines[0] != "#!/bin/sh" {
		t.Errorf("shebang moved: %q", lines[0])
	}
	if !strings.Contains(lines[1], launchHookMarker) || !strings.Contains(lines[1], `"$1"`) {
		t.Errorf("hook not on line 2: %q", lines[1])
	}
	if lines[2] != "echo $0 $*" {
		t.Errorf("original body disturbed: %q", lines[2])
	}
}

func TestInstallTwiceChangesNothingTheSecondTime(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	if _, err := installLaunchHook(emu, "/s.sh"); err != nil {
		t.Fatal(err)
	}
	once := read(t, path)

	changed, err := installLaunchHook(emu, "/s.sh")
	if err != nil || changed != 0 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	if read(t, path) != once {
		t.Error("second install rewrote the file")
	}
	if n := strings.Count(once, launchHookMarker); n != 1 {
		t.Errorf("hook appears %d times", n)
	}
}

func TestRemoveRestoresTheOriginalExactly(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "GB", gbLaunch, 0755)

	installLaunchHook(emu, "/s.sh")
	changed, err := removeLaunchHook(emu)
	if err != nil || changed != 1 {
		t.Fatalf("changed=%d err=%v", changed, err)
	}
	if got := read(t, path); got != gbLaunch {
		t.Errorf("not restored:\n%q", got)
	}
}

func TestInstallKeepsPermissionsAndLineEndings(t *testing.T) {
	emu := t.TempDir()
	crlf := strings.ReplaceAll(gbLaunch, "\n", "\r\n")
	path := writeEmu(t, emu, "GBA", crlf, 0755)

	installLaunchHook(emu, "/s.sh")

	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0755 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	got := read(t, path)
	if strings.Contains(strings.ReplaceAll(got, "\r\n", ""), "\n") {
		t.Error("mixed line endings")
	}

	removeLaunchHook(emu)
	if read(t, path) != crlf {
		t.Error("CRLF file not restored")
	}
}

func TestScriptWithoutShebangGetsTheHookOnTop(t *testing.T) {
	emu := t.TempDir()
	path := writeEmu(t, emu, "X", "echo hi\n", 0755)

	installLaunchHook(emu, "/s.sh")

	if first := strings.SplitN(read(t, path), "\n", 2)[0]; !strings.Contains(first, launchHookMarker) {
		t.Errorf("first line = %q", first)
	}
}

func TestHookRunsTheScriptOnlyWhenItIsThere(t *testing.T) {
	line := hookLine("/mnt/SDCARD/App/Grout/sync_on_launch.sh")
	if !strings.HasPrefix(line, `if [ -f "/mnt/SDCARD/App/Grout/sync_on_launch.sh" ]; then sh `) || !strings.HasSuffix(line, "fi "+launchHookMarker) {
		t.Errorf("line = %q", line)
	}
}

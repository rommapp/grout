package host

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func rooted(root string) func(string) string {
	return func(k string) string {
		if k == "GROUT_ROOT" {
			return root
		}
		return ""
	}
}

func TestOSHostReadsUnderGroutRoot(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "proc/bus/input"), 0755)
	os.WriteFile(filepath.Join(root, "proc/bus/input/devices"), []byte("TRIMUI"), 0644)
	h := OS(rooted(root))
	b, err := h.ReadFile("/proc/bus/input/devices")
	if err != nil || string(b) != "TRIMUI" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := h.Stat("/opt/muos"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("leaked outside root: %v", err)
	}
}

func TestOSHostCannotClimbOutOfRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	os.MkdirAll(filepath.Join(root, "etc"), 0755)
	os.WriteFile(filepath.Join(root, "etc/passwd"), []byte("inside"), 0644)
	os.MkdirAll(filepath.Join(base, "etc"), 0755)
	os.WriteFile(filepath.Join(base, "etc/passwd"), []byte("outside"), 0644)
	h := OS(rooted(root))
	b, err := h.ReadFile("/../../etc/passwd")
	if err != nil || string(b) != "inside" {
		t.Fatalf("%q %v", b, err)
	}
}

func TestOSHostRelativePathIsError(t *testing.T) {
	h := OS(rooted(t.TempDir()))
	if _, err := h.ReadFile("etc/passwd"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := h.Stat("etc"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := h.ReadDir("etc"); err == nil {
		t.Fatal("expected error")
	}
}

func TestOSHostReadDirAndStat(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "mnt/sdcard/Roms"), 0755)
	h := OS(rooted(root))
	entries, err := h.ReadDir("/mnt/sdcard")
	if err != nil || len(entries) != 1 || entries[0].Name() != "Roms" {
		t.Fatalf("%v %v", entries, err)
	}
	fi, err := h.Stat("/mnt/sdcard/Roms")
	if err != nil || !fi.IsDir() {
		t.Fatalf("%v %v", fi, err)
	}
}

func TestOSHostWithoutRootUsesPathsAsIs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	os.WriteFile(p, []byte("x"), 0644)
	h := OS(func(string) string { return "" })
	b, err := h.ReadFile(p)
	if err != nil || string(b) != "x" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := h.ReadFile(filepath.Join(dir, "nope")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%v", err)
	}
}

func TestOSHostGetenv(t *testing.T) {
	h := OS(func(k string) string {
		if k == "A" {
			return "b"
		}
		return ""
	})
	if h.Getenv("A") != "b" || h.Getenv("Z") != "" {
		t.Fatal("getenv")
	}
}

func TestOSHostExecutableWithoutRoot(t *testing.T) {
	want, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	got, err := OS(func(string) string { return "" }).Executable()
	if err != nil || got != want {
		t.Fatalf("%q %q %v", got, want, err)
	}
}

func TestOSHostExecutableStripsRoot(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	h := OS(rooted(filepath.Dir(exe)))
	got, err := h.Executable()
	if err != nil || got != "/"+filepath.Base(exe) {
		t.Fatalf("%q %v", got, err)
	}
}

func TestOSHostExecutableOutsideRootUnchanged(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skip(err)
	}
	got, err := OS(rooted(t.TempDir())).Executable()
	if err != nil || got != exe {
		t.Fatalf("%q %v", got, err)
	}
}

func TestFakeHost(t *testing.T) {
	h := &Fake{Files: fstest.MapFS{"opt/x": {Data: []byte("1")}}, Env: map[string]string{"A": "b"}}
	if b, _ := h.ReadFile("/opt/x"); string(b) != "1" || h.Getenv("A") != "b" || h.Getenv("Z") != "" {
		t.Fatal("fake host")
	}
}

func TestFakeHostPaths(t *testing.T) {
	h := &Fake{Files: fstest.MapFS{
		"opt/x":   {Data: []byte("1")},
		"opt/y/z": {Data: []byte("2")},
		"etc/p":   {Data: []byte("in")},
	}}
	if b, err := h.ReadFile("/../../etc/p"); err != nil || string(b) != "in" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := h.ReadFile("/missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%v", err)
	}
	if _, err := h.ReadFile("opt/x"); err == nil {
		t.Fatal("expected error for relative path")
	}
	fi, err := h.Stat("/opt/y")
	if err != nil || !fi.IsDir() {
		t.Fatalf("%v %v", fi, err)
	}
	entries, err := h.ReadDir("/opt")
	if err != nil || len(entries) != 2 {
		t.Fatalf("%v %v", entries, err)
	}
	if _, err := h.ReadDir("/"); err != nil {
		t.Fatal(err)
	}
}

func TestFakeHostExecutable(t *testing.T) {
	if _, err := (&Fake{}).Executable(); err == nil {
		t.Fatal("expected error")
	}
	got, err := (&Fake{Exe: "/mnt/x/grout"}).Executable()
	if err != nil || got != "/mnt/x/grout" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestStripRootHandlesSymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	os.MkdirAll(filepath.Join(real, "mnt/sdcard/app"), 0755)
	link := filepath.Join(base, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip(err)
	}
	h := OS(rooted(link)).(*osHost)
	if len(h.roots) != 2 {
		t.Fatalf("%v", h.roots)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{
		filepath.Join(link, "mnt/sdcard/app/grout"),
		filepath.Join(resolved, "mnt/sdcard/app/grout"),
	} {
		if got := stripRoot(exe, h.roots); got != "/mnt/sdcard/app/grout" {
			t.Fatalf("%q -> %q", exe, got)
		}
	}
	if got := stripRoot("/usr/bin/grout", h.roots); got != "/usr/bin/grout" {
		t.Fatal(got)
	}
}

func TestStripRootCleansRoot(t *testing.T) {
	h := OS(rooted("/tmp/x/../grout-root/")).(*osHost)
	if got := stripRoot("/tmp/grout-root/a/b", h.roots); got != "/a/b" {
		t.Fatal(got)
	}
}

func TestOSHostRealAppliesRoot(t *testing.T) {
	h := OS(rooted("/tmp/x"))
	got, err := h.Real("/mnt/a")
	if err != nil || got != filepath.Join("/tmp/x", "mnt", "a") {
		t.Fatalf("%q %v", got, err)
	}
	if _, err := h.Real("mnt/a"); err == nil {
		t.Fatal("expected error")
	}
}

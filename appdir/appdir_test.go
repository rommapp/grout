package appdir

import (
	"os"
	"path/filepath"
	"testing"
)

func unsetAll(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GROUT_DATA_DIR", "GROUT_CACHE_DIR", "GROUT_TMP_DIR", "GROUT_UPDATE_DIR"} {
		t.Setenv(name, "")
	}
}

// Unset, every location is where grout kept it before it could be moved, so an
// existing install sees no change.
func TestDefaultsAreWhereGroutAlwaysKeptThem(t *testing.T) {
	unsetAll(t)
	t.Chdir(t.TempDir())
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string][2]string{
		"DataDir":          {DataDir(), wd},
		"CacheDir":         {CacheDir(), filepath.Join(wd, ".cache")},
		"TmpDir":           {TmpDir(), filepath.Join(wd, ".tmp")},
		"SystemTmpDir":     {SystemTmpDir(), os.TempDir()},
		"UpdateStagingDir": {UpdateStagingDir("/install"), filepath.Join("/install", ".update")},
	}
	for name, c := range cases {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

// Moving the data directory carries the cache and the download staging area
// with it, unless they are moved on their own.
func TestDataDirCarriesTheCacheAndStaging(t *testing.T) {
	unsetAll(t)
	data := t.TempDir()
	t.Setenv("GROUT_DATA_DIR", data)

	if got, want := CacheDir(), filepath.Join(data, ".cache"); got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
	if got, want := TmpDir(), filepath.Join(data, ".tmp"); got != want {
		t.Errorf("TmpDir = %q, want %q", got, want)
	}
}

func TestOverridesWin(t *testing.T) {
	unsetAll(t)
	t.Setenv("GROUT_DATA_DIR", "/data")
	t.Setenv("GROUT_CACHE_DIR", "/cache")
	t.Setenv("GROUT_TMP_DIR", "/tmp/grout")
	t.Setenv("GROUT_UPDATE_DIR", "/tmp/grout/update")

	cases := map[string][2]string{
		"DataDir":          {DataDir(), "/data"},
		"CacheDir":         {CacheDir(), "/cache"},
		"TmpDir":           {TmpDir(), "/tmp/grout"},
		"SystemTmpDir":     {SystemTmpDir(), "/tmp/grout"},
		"UpdateStagingDir": {UpdateStagingDir("/install"), "/tmp/grout/update"},
	}
	for name, c := range cases {
		if c[0] != c[1] {
			t.Errorf("%s = %q, want %q", name, c[0], c[1])
		}
	}
}

package leaf

import (
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"grout/core/firmware/host"
)

func dir() *fstest.MapFile {
	return &fstest.MapFile{Mode: fs.ModeDir | 0o755}
}

func TestCardRoot(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		exe  string
		fs   fstest.MapFS
		want string
	}{
		{"SDCARD_PATH wins", map[string]string{"SDCARD_PATH": "/sd"}, "/media/sdcard1/Apps/mlp1/Grout.pak/grout", fstest.MapFS{"media/sdcard1/Roms": dir()}, "/sd"},
		{"executable four levels below a card with Roms", nil, "/media/sdcard1/Apps/mlp1/Grout.pak/grout", fstest.MapFS{"media/sdcard1/Roms": dir()}, "/media/sdcard1"},
		{"executable on the primary card", nil, "/mnt/sdcard/Apps/mlp1/Grout.pak/grout", fstest.MapFS{"mnt/sdcard/Roms": dir()}, "/mnt/sdcard"},
		{"no Roms at the walked root", nil, "/media/sdcard1/Apps/mlp1/Grout.pak/grout", fstest.MapFS{"media/sdcard1/Apps": dir()}, "/mnt/sdcard"},
		{"Roms is a file", nil, "/media/sdcard1/Apps/mlp1/Grout.pak/grout", fstest.MapFS{"media/sdcard1/Roms": &fstest.MapFile{Data: []byte("x")}}, "/mnt/sdcard"},
		{"Roms one level too high", nil, "/media/sdcard1/Apps/mlp1/Grout.pak/grout", fstest.MapFS{"media/Roms": dir()}, "/mnt/sdcard"},
		{"no executable", nil, "", fstest.MapFS{}, "/mnt/sdcard"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &host.Fake{Files: tc.fs, Env: tc.env, Exe: tc.exe}
			got, err := CardRoot(h, map[string]string{})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("CardRoot = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCardRootMockEnv(t *testing.T) {
	data, err := os.ReadFile("mock.env")
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			env[k] = v
		}
	}
	got, err := CardRoot(&host.Fake{Files: fstest.MapFS{}, Env: env}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "/mnt/sdcard" {
		t.Errorf("CardRoot = %q, want /mnt/sdcard", got)
	}
}

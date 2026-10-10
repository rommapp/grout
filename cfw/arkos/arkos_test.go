package arkos

import (
	"os"
	"path/filepath"
	"testing"
)

func withFstab(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fstab")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := fstabPath
	fstabPath = path
	t.Cleanup(func() { fstabPath = previous })
}

const mainCardFstab = `LABEL=EASYROMS /roms vfat defaults,auto,umask=000,uid=1000,gid=1000,noatime 0 0
/roms/tools /opt/system/Tools none bind 0 0
`

func TestGetBasePath(t *testing.T) {
	tests := []struct {
		name  string
		fstab string
		want  string
	}{
		{"main card", mainCardFstab, "/roms"},
		{
			"switched to SD2",
			`LABEL=EASYROMS /roms vfat defaults,auto,umask=000,uid=1000,gid=1000,noatime 0 0
/roms2/tools /opt/system/Tools none bind 0 0
/dev/mmcblk1p1 /roms2 exfat umask=0000,iocharset=utf8,noatime,nofail,x-systemd.device-timeout=7,uid=1000,gid=1000 0 0
`,
			"/roms2",
		},
		{"only the tools bind points at SD2", "/roms2/tools /opt/system/Tools none bind 0 0\n", "/roms"},
		{"commented out SD2 mount", mainCardFstab + "#/dev/mmcblk1p1 /roms2 exfat defaults 0 0\n", "/roms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BASE_PATH", "")
			withFstab(t, tt.fstab)
			if got := GetBasePath(); got != tt.want {
				t.Errorf("GetBasePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBasePath_MissingFstabFallsBackToMainCard(t *testing.T) {
	t.Setenv("BASE_PATH", "")
	fstabPath = filepath.Join(t.TempDir(), "missing")
	t.Cleanup(func() { fstabPath = "/etc/fstab" })
	if got := GetBasePath(); got != "/roms" {
		t.Errorf("GetBasePath() = %q, want /roms", got)
	}
}

func TestGetBasePath_EnvOverridesFstab(t *testing.T) {
	t.Setenv("BASE_PATH", "/custom")
	withFstab(t, "/dev/mmcblk1p1 /roms2 exfat defaults 0 0\n")
	if got := GetBasePath(); got != "/custom" {
		t.Errorf("GetBasePath() = %q, want /custom", got)
	}
}

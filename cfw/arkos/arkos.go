package arkos

import (
	"bufio"
	"embed"
	"grout/tables"
	"os"
	"path/filepath"
	"strings"
)

//go:embed data/*.json
var embeddedFiles embed.FS

var (
	Platforms = tables.MustLoad[string, []string](embeddedFiles, "data/platforms.json")
)

const (
	mainCardPath = "/roms"
	sd2CardPath  = "/roms2"
)

// fstabPath is a variable so tests can point it at a fixture.
var fstabPath = "/etc/fstab"

// GetBasePath returns the ROM root EmulationStation is using. BASE_PATH, when set, always wins.
// Otherwise ROMs are on SD2 when "Switch to SD2 for Roms" has been run: that script adds a /roms2
// mount to /etc/fstab (and "Switch to Main SD for Roms" removes it) but leaves /roms mounted, so
// /roms existing proves nothing. Checking fstab is the same test PortMaster uses on ArkOS/dArkOS:
// https://github.com/PortsMaster/PortMaster-GUI/blob/6b2feea02ad031f9842513e97d49788d80f58716/PortMaster/control.txt#L18-L26
func GetBasePath() string {
	if basePath := os.Getenv("BASE_PATH"); basePath != "" {
		return basePath
	}
	if fstabMountsSD2() {
		return sd2CardPath
	}
	return mainCardPath
}

// fstabMountsSD2 reports whether fstab has an entry mounting /roms2. Only the mount point field
// is matched, so the /roms2/tools bind line the switch script also writes does not count.
func fstabMountsSD2() bool {
	f, err := os.Open(fstabPath)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if filepath.Clean(fields[1]) == sd2CardPath {
			return true
		}
	}
	return false
}

func GetRomDirectory() string {
	return GetBasePath()
}

func GetBIOSDirectory() string {
	return filepath.Join(GetBasePath(), "bios")
}

func GetBaseSavePath() string {
	return GetRomDirectory()
}

func GetArtDirectory(romDir string) string {
	return filepath.Join(romDir, "images")
}

func GetGroutGamelist() string {
	return filepath.Join(GetRomDirectory(), "ports", "gamelist.xml")
}

func GetVideoDirectory(romDir string) string {
	return filepath.Join(romDir, "videos")
}

func GetManualDirectory(romDir string) string {
	return filepath.Join(romDir, "manuals")
}

func GetBezelDirectory(romDir string) string {
	return filepath.Join(romDir, "bezels")
}

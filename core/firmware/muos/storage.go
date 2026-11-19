package muos

import (
	"path"
	"strings"

	"grout/core/firmware/host"
)

const (
	unionRoms        = "/mnt/union/ROMS"
	storageConfigDir = "/opt/muos/device/config/storage"
	storageSD1       = "rom"
)

var unionOrder = []struct {
	storage  string
	fallback string
}{
	{"usb", "/mnt/usb"},
	{"sdcard", "/mnt/sdcard"},
	{storageSD1, "/mnt/mmc"},
}

func RomRoot(h host.Host, vars map[string]string) (string, error) {
	if isDir(h, unionRoms) {
		return unionRoms, nil
	}
	preferred := vars["muos_rom_storage"]
	candidates := make([]string, 0, len(unionOrder))
	for _, entry := range unionOrder {
		roms := path.Join(mountPoint(h, entry.storage), "ROMS")
		if entry.storage == preferred {
			candidates = append([]string{roms}, candidates...)
		} else {
			candidates = append(candidates, roms)
		}
	}
	for _, roms := range candidates {
		if isDir(h, roms) {
			return roms, nil
		}
	}
	return path.Join(mountPoint(h, storageSD1), "ROMS"), nil
}

func mountPoint(h host.Host, storage string) string {
	fallback := ""
	for _, entry := range unionOrder {
		if entry.storage == storage {
			fallback = entry.fallback
		}
	}
	data, err := h.ReadFile(path.Join(storageConfigDir, storage, "mount"))
	if err != nil {
		return fallback
	}
	if mount := strings.TrimSpace(string(data)); mount != "" {
		return mount
	}
	return fallback
}

func isDir(h host.Host, p string) bool {
	info, err := h.Stat(p)
	return err == nil && info.IsDir()
}

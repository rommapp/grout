package leaf

import (
	"path"

	"grout/core/firmware/host"
)

const (
	defaultCard  = "/mnt/sdcard"
	installDepth = 4
)

func CardRoot(h host.Host, vars map[string]string) (string, error) {
	if sd := h.Getenv("SDCARD_PATH"); sd != "" {
		return sd, nil
	}
	if root := cardFromExecutable(h); root != "" {
		return root, nil
	}
	return defaultCard, nil
}

func cardFromExecutable(h host.Host) string {
	exe, err := h.Executable()
	if err != nil || !path.IsAbs(exe) {
		return ""
	}
	root := exe
	for range installDepth {
		root = path.Dir(root)
	}
	info, err := h.Stat(path.Join(root, "Roms"))
	if err != nil || !info.IsDir() {
		return ""
	}
	return root
}

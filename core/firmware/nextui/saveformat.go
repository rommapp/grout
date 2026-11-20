package nextui

import (
	"path"
	"strings"

	"github.com/rommapp/grout/core/firmware/host"
)

const (
	saveFormatMinUI                 = "0"
	saveFormatRetroArch             = "1"
	saveFormatGeneric               = "2"
	saveFormatRetroArchUncompressed = "3"
)

func SaveFormat(h host.Host, vars map[string]string) (keepRomExt bool, ext string, ok bool) {
	switch saveFormat(h, vars["card"]) {
	case saveFormatRetroArch, saveFormatRetroArchUncompressed:
		return false, "srm", true
	case saveFormatGeneric:
		return false, "sav", true
	default:
		return true, "sav", true
	}
}

func saveFormat(h host.Host, card string) string {
	data, err := h.ReadFile(path.Join("/", card, ".userdata", "shared", "minuisettings.txt"))
	if err != nil {
		return saveFormatMinUI
	}
	format := saveFormatMinUI
	for _, line := range strings.Split(string(data), "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "saveFormat="); found {
			format = value
		}
	}
	return format
}

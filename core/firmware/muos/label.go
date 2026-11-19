package muos

import (
	"path"
	"strings"
)

func Label(saveFolder string) string {
	if rest, ok := strings.CutPrefix(saveFolder, picklesSaveRoot+"/"); ok {
		core, _, _ := strings.Cut(rest, "/")
		return "Pickles (" + core + ")"
	}
	trimmed := strings.TrimPrefix(saveFolder, "file/")
	trimmed = strings.TrimSuffix(trimmed, "/backup")
	return path.Base(trimmed)
}

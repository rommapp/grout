package firmware

import (
	_ "embed"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

//go:embed bios_subfolders.toml
var biosSubfoldersTOML []byte

var biosSubfolders = sync.OnceValues(func() (map[string][]string, error) {
	var doc struct {
		Src   string              `toml:"src"`
		Files map[string][]string `toml:"files"`
	}
	md, err := toml.Decode(string(biosSubfoldersTOML), &doc)
	if err != nil {
		return nil, fmt.Errorf("bios_subfolders.toml: %w", err)
	}
	if un := md.Undecoded(); len(un) > 0 {
		return nil, fmt.Errorf("bios_subfolders.toml: unknown keys: %v", un)
	}
	return doc.Files, nil
})

func BiosRels(fileName string) []string {
	base := path.Base(fileName)
	table, err := biosSubfolders()
	if err == nil {
		if rels, ok := table[strings.ToLower(base)]; ok {
			return append([]string(nil), rels...)
		}
	}
	return []string{base}
}

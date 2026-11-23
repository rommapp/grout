package firmware

import (
	"fmt"
	"strings"

	"github.com/BurntSushi/toml"
)

var factKeys = map[string]bool{"value": true, "src": true, "since": true, "until": true, "hook": true}

var factPaths = map[string]bool{
	"version.file": true, "paths.card": true, "paths.roms": true, "paths.bios": true, "paths.saves": true,
	"rom_folders.style": true, "saves.layout": true, "saves.naming": true, "bios.default": true,
	"art.cover": true, "art.preview": true, "art.splash": true, "art.video": true, "art.manual": true, "art.bezel": true,
	"retroarch.config": true, "retroarch.overrides": true, "gamelist.text": true, "exit_actions.command": true,
}

func Parse(data []byte) (*Definition, error) {
	var d Definition
	md, err := toml.Decode(string(data), &d)
	if err != nil {
		return nil, err
	}
	un := md.Undecoded()
	var keys []string
	for _, k := range un {
		if len(k) > 1 && factKeys[k[len(k)-1]] && factPaths[k[:len(k)-1].String()] {
			continue
		}
		keys = append(keys, k.String())
	}
	if len(keys) > 0 {
		return nil, fmt.Errorf("unknown keys: %s", strings.Join(keys, ", "))
	}
	return &d, nil
}

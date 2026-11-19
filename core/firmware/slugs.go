package firmware

import (
	_ "embed"
	"fmt"

	"github.com/BurntSushi/toml"
)

//go:embed romm_platforms.toml
var rommPlatforms []byte

type rommPlatformList struct {
	Upstream struct {
		Repo string `toml:"repo"`
		Ref  string `toml:"ref"`
	} `toml:"upstream"`
	Src   string   `toml:"src"`
	Slugs []string `toml:"slugs"`
	Extra map[string]struct {
		Reason string `toml:"reason"`
	} `toml:"extra"`
}

func KnownSlugs() (map[string]bool, error) {
	var list rommPlatformList
	md, err := toml.Decode(string(rommPlatforms), &list)
	if err != nil {
		return nil, err
	}
	if un := md.Undecoded(); len(un) > 0 {
		return nil, fmt.Errorf("romm_platforms.toml: unknown keys: %v", un)
	}
	known := make(map[string]bool, len(list.Slugs)+len(list.Extra))
	for _, s := range list.Slugs {
		known[s] = true
	}
	for s := range list.Extra {
		known[s] = true
	}
	return known, nil
}

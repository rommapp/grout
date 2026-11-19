package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	v2ConfigFile = "config.json"
	v2SlotsFile  = "save_slots.json"
)

func importV2(dir string) (*Config, bool, error) {
	path := filepath.Join(dir, v2ConfigFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, fmt.Errorf("reading %s: %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, true, fmt.Errorf("parsing %s: %w", path, err)
	}
	for slug, saveDir := range c.SaveDirectoryMappings {
		if strings.HasPrefix(saveDir, "pickles/sram/") {
			c.SaveDirectoryMappings[slug] = "../" + saveDir
		}
	}
	c.ChannelPreferences = readV2Slots(filepath.Join(dir, v2SlotsFile))
	applyDefaults(&c)
	return &c, true, nil
}

func readV2Slots(path string) map[int]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var slots map[string]string
	if json.Unmarshal(data, &slots) != nil {
		return nil
	}
	prefs := make(map[int]string, len(slots))
	for key, label := range slots {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		if label == "autosave" {
			label = "default"
		}
		prefs[id] = label
	}
	if len(prefs) == 0 {
		return nil
	}
	return prefs
}

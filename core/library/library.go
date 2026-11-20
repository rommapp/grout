package library

import (
	"fmt"
	"os"
	"path/filepath"

	"grout/core/config"
	"grout/core/firmware"
	"grout/core/romm"
	"grout/core/store"
)

const (
	StagePlatforms   = "platforms"
	StageGames       = "games"
	StageCollections = "collections"
	StagePurge       = "purge"
	StageScan        = "scan"
)

type Progress struct {
	Stage string
	Done  int
	Total int
}

type Library struct {
	Store    *store.Store
	Client   *romm.Client
	Config   *config.Config
	Firmware *firmware.Firmware
}

func New(s *store.Store, c *romm.Client, cfg *config.Config, fw *firmware.Firmware) *Library {
	return &Library{Store: s, Client: c, Config: cfg, Firmware: fw}
}

func Open(dir string) (*store.Store, error) {
	cacheDir := filepath.Join(dir, ".cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating %s: %w", cacheDir, err)
	}
	return store.Open(filepath.Join(cacheDir, store.FileName), Tables())
}

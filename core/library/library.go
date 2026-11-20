package library

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/rommapp/grout/core/config"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/romm"
	"github.com/rommapp/grout/core/store"
)

type Stage string

const (
	StagePlatforms   Stage = "platforms"
	StageGames       Stage = "games"
	StageCollections Stage = "collections"
	StagePurge       Stage = "purge"
	StageScan        Stage = "scan"
)

type Progress struct {
	Stage Stage
	Done  int
	Total int
}

type Library struct {
	Store    *store.Store
	Client   *romm.Client
	Config   *config.Config
	Firmware *firmware.Firmware
	Log      *slog.Logger
}

func (l *Library) log() *slog.Logger {
	if l.Log == nil {
		return slog.New(slog.DiscardHandler)
	}
	return l.Log
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

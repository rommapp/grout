package models

import "github.com/brandonkowalski/go-romm"

type AppState struct {
	Config      *Config
	HostIndices map[string]int

	CurrentFullGamesList []romm.DetailedRom
	LastSelectedIndex    int
	LastSelectedPosition int
}

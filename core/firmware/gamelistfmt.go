package firmware

import "path"

type GamelistFormat string

const (
	FormatNone             GamelistFormat = "none"
	FormatEmulationStation GamelistFormat = "emulationstation"
	FormatMiyoo            GamelistFormat = "miyoo"
	FormatMuOSText         GamelistFormat = "muos-text"
)

func GamelistFormats() []GamelistFormat {
	return []GamelistFormat{FormatNone, FormatEmulationStation, FormatMiyoo, FormatMuOSText}
}

func (f *Firmware) GamelistFormat() GamelistFormat {
	if f.Def.Gamelist.Format == "" {
		return FormatNone
	}
	return f.Def.Gamelist.Format
}

func (f *Firmware) GamelistTextDir(slug, romDir, platformName string) (string, bool) {
	return f.expandArtFact(f.Def.Gamelist.Text, slug, romDir, platformName)
}

type Launcher struct {
	Gamelist string
	Path     string
}

func (f *Firmware) LauncherEntry() (Launcher, bool) {
	le := f.Def.Gamelist.LauncherEntry
	if le == nil {
		return Launcher{}, false
	}
	vars := map[string]string{"card": f.Card, "roms": f.Roms}
	v, err := ExpandEnv(le.Gamelist, vars, f.Host.Getenv)
	if err != nil {
		return Launcher{}, false
	}
	return Launcher{Gamelist: path.Clean(v), Path: le.Path}, true
}

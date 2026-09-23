package saves

import (
	"slices"
	"strings"

	"grout/cache"
	"grout/cfw"
	"grout/settings"
)

// EmulatorChoice is a platform whose saves could go in more than one place.
type EmulatorChoice struct {
	FSSlug string
	// Name is the platform as the user knows it.
	Name string
	// Directories are the emulator folders the firmware knows for it, the
	// first being the one it uses by default.
	Directories []string
	// Labels are those folders as they should read on screen.
	Labels []string
	// Chosen is the folder currently mapped, or the default when none is.
	Chosen string
}

// EmulatorChoices lists the platforms worth asking about: ones the user has
// mapped a rom folder for, whose firmware keeps saves in more than one place.
//
// A platform with a single folder has no choice to offer, so it is left out
// rather than shown with one option.
func EmulatorChoices(config settings.Config) []EmulatorChoice {
	activeCFW := cfw.GetCFW()

	folders := cfw.EmulatorFolderMap(activeCFW)
	if folders == nil {
		return nil
	}

	names := platformNames()

	var choices []EmulatorChoice
	for fsSlug := range config.DirectoryMappings {
		directories := folders[config.ResolveFSSlug(fsSlug)]
		if len(directories) < 2 {
			continue
		}

		labels := make([]string, 0, len(directories))
		for _, dir := range directories {
			labels = append(labels, cfw.EmulatorLabel(activeCFW, dir))
		}

		choices = append(choices, EmulatorChoice{
			FSSlug:      fsSlug,
			Name:        platformName(fsSlug, names, config),
			Directories: directories,
			Labels:      labels,
			Chosen:      chosenEmulator(config, fsSlug, directories),
		})
	}

	slices.SortFunc(choices, func(a, b EmulatorChoice) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return choices
}

// chosenEmulator is the folder a platform's saves go in: what the user picked,
// or the firmware's default when they have not picked.
func chosenEmulator(config settings.Config, fsSlug string, directories []string) string {
	if mapped, ok := config.SaveDirectoryMappings[fsSlug]; ok && slices.Contains(directories, mapped) {
		return mapped
	}
	return directories[0]
}

// ChooseEmulator records which folder a platform's saves belong in.
//
// A platform left on the firmware's default is removed rather than recorded,
// so it keeps following that default instead of holding a copy of what it was
// on the day it was chosen.
func ChooseEmulator(config *settings.Config, fsSlug, directory string, directories []string) {
	if len(directories) > 0 && directory == directories[0] {
		delete(config.SaveDirectoryMappings, fsSlug)
		if len(config.SaveDirectoryMappings) == 0 {
			config.SaveDirectoryMappings = nil
		}
		return
	}

	if config.SaveDirectoryMappings == nil {
		config.SaveDirectoryMappings = make(map[string]string)
	}
	config.SaveDirectoryMappings[fsSlug] = directory
}

// platformNames is what the cache calls each platform, so the screen shows the
// same names the rest of grout does.
func platformNames() map[string]string {
	manager := cache.GetCacheManager()
	if manager == nil {
		return nil
	}

	platforms, err := manager.GetPlatforms()
	if err != nil {
		return nil
	}

	names := make(map[string]string, len(platforms))
	for _, platform := range platforms {
		names[platform.FSSlug] = platform.Name
	}
	return names
}

// platformName falls back through what is known about a platform, ending at
// its slug, which is always something rather than a blank row.
func platformName(fsSlug string, names map[string]string, config settings.Config) string {
	if name, ok := names[fsSlug]; ok && name != "" {
		return name
	}
	if mapping, ok := config.DirectoryMappings[fsSlug]; ok && mapping.RomMSlug != "" {
		return mapping.RomMSlug
	}
	return fsSlug
}

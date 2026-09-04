package saves

import (
	"testing"

	"grout/cfw"
	"grout/settings"
)

func mappedTo(slugs ...string) map[string]settings.DirectoryMapping {
	m := make(map[string]settings.DirectoryMapping, len(slugs))
	for _, slug := range slugs {
		m[slug] = settings.DirectoryMapping{RomMSlug: slug, RelativePath: slug}
	}
	return m
}

// A platform whose firmware keeps saves in one place has nothing to ask about,
// and showing it with a single option would be offering a choice that is not
// one.
func TestEmulatorChoices_SkipsPlatformsWithOnePlace(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	folders := cfw.EmulatorFolderMap(cfw.MuOS)
	single, several := "", ""
	for slug, dirs := range folders {
		if len(dirs) == 1 && single == "" {
			single = slug
		}
		if len(dirs) > 1 && several == "" {
			several = slug
		}
	}
	if single == "" || several == "" {
		t.Skip("muOS has no platform of each kind to compare")
	}

	choices := EmulatorChoices(settings.Config{DirectoryMappings: mappedTo(single, several)})

	for _, choice := range choices {
		if choice.FSSlug == single {
			t.Errorf("%q has one save folder and must not be offered", single)
		}
	}
	found := false
	for _, choice := range choices {
		if choice.FSSlug == several {
			found = true
		}
	}
	if !found {
		t.Errorf("%q has several save folders and must be offered", several)
	}
}

// Only platforms the user has a rom folder for are worth asking about.
func TestEmulatorChoices_OnlyMappedPlatforms(t *testing.T) {
	t.Setenv(cfw.EnvVar, string(cfw.MuOS))

	if choices := EmulatorChoices(settings.Config{}); len(choices) != 0 {
		t.Errorf("got %d choices with nothing mapped, want none", len(choices))
	}
}

func TestChooseEmulator_RecordsANonDefault(t *testing.T) {
	config := &settings.Config{}
	directories := []string{"default/dir", "other/dir"}

	ChooseEmulator(config, "psp", "other/dir", directories)

	if got := config.SaveDirectoryMappings["psp"]; got != "other/dir" {
		t.Errorf("mapping = %q, want the chosen folder", got)
	}
}

// A platform left on the firmware's default is not recorded, so it keeps
// following that default rather than holding a copy of what it was on the day
// it was chosen.
func TestChooseEmulator_DefaultIsNotRecorded(t *testing.T) {
	directories := []string{"default/dir", "other/dir"}
	config := &settings.Config{SaveDirectoryMappings: map[string]string{"psp": "other/dir"}}

	ChooseEmulator(config, "psp", "default/dir", directories)

	if _, recorded := config.SaveDirectoryMappings["psp"]; recorded {
		t.Error("choosing the default left a mapping behind")
	}
}

// An empty map and no map mean the same thing, and one of them is what gets
// written to the config file.
func TestChooseEmulator_ClearsAnEmptyMap(t *testing.T) {
	directories := []string{"default/dir", "other/dir"}
	config := &settings.Config{SaveDirectoryMappings: map[string]string{"psp": "other/dir"}}

	ChooseEmulator(config, "psp", "default/dir", directories)

	if config.SaveDirectoryMappings != nil {
		t.Errorf("mappings = %v, want nothing left", config.SaveDirectoryMappings)
	}
}

// Choosing one platform must not disturb another.
func TestChooseEmulator_LeavesOthersAlone(t *testing.T) {
	directories := []string{"default/dir", "other/dir"}
	config := &settings.Config{SaveDirectoryMappings: map[string]string{"psx": "kept/dir"}}

	ChooseEmulator(config, "psp", "other/dir", directories)

	if got := config.SaveDirectoryMappings["psx"]; got != "kept/dir" {
		t.Errorf("psx mapping = %q, want it untouched", got)
	}
}

func TestChosenEmulator(t *testing.T) {
	directories := []string{"default/dir", "other/dir"}

	// Nothing recorded means the firmware's default.
	if got := chosenEmulator(settings.Config{}, "psp", directories); got != "default/dir" {
		t.Errorf("chosen = %q, want the default", got)
	}

	config := settings.Config{SaveDirectoryMappings: map[string]string{"psp": "other/dir"}}
	if got := chosenEmulator(config, "psp", directories); got != "other/dir" {
		t.Errorf("chosen = %q, want what was recorded", got)
	}

	// A folder the firmware no longer has, from an upgrade or a different
	// device, cannot be shown and must not leave the row on nothing.
	stale := settings.Config{SaveDirectoryMappings: map[string]string{"psp": "gone/dir"}}
	if got := chosenEmulator(stale, "psp", directories); got != "default/dir" {
		t.Errorf("chosen = %q, want the default for a folder that no longer exists", got)
	}
}

func TestPlatformName(t *testing.T) {
	config := settings.Config{DirectoryMappings: mappedTo("psp")}

	if got := platformName("psp", map[string]string{"psp": "PlayStation Portable"}, config); got != "PlayStation Portable" {
		t.Errorf("name = %q, want what the cache calls it", got)
	}
	// No cache entry falls back to what the mapping recorded.
	if got := platformName("psp", nil, config); got != "psp" {
		t.Errorf("name = %q, want the mapping's slug", got)
	}
	// Nothing known at all still gives the row something to say.
	if got := platformName("unknown", nil, settings.Config{}); got != "unknown" {
		t.Errorf("name = %q, want the slug itself", got)
	}
}

package catalog

import (
	"grout/cfw"
	"grout/romm"
	"grout/settings"
	"grout/textmatch"
)

// DirectoryChoice is one rom folder a platform can be mapped to.
type DirectoryChoice struct {
	// RelativePath is stored in the mapping and joined to the rom root.
	RelativePath string
	// Display is what to show. MinUI and NextUI tag their folders, and the tag
	// is what tells two of them apart, so "Game Boy Advance (MGBA)" shows as
	// "MGBA".
	Display string
	// Create marks a folder the firmware expects but that is not on disk yet.
	Create bool
}

// ChoiceRequest asks which folders a platform could map to.
type ChoiceRequest struct {
	Platform romm.Platform
	CFW      cfw.CFW
	// Directories are the folder names present under the rom root.
	Directories []string
	// PlatformsBinding is the server's own slug mapping.
	PlatformsBinding map[string]string
	// Existing carries the mappings from a previous visit. Empty means a first
	// run, where a folder is matched by name instead.
	Existing map[string]settings.DirectoryMapping
	// AutoSelect offers to create the firmware's own folder when nothing on
	// disk matched.
	AutoSelect bool
}

// Choices is what a platform can map to and what should start selected.
type Choices struct {
	Directories []DirectoryChoice
	// Selected indexes Directories, or -1 when nothing matched and the platform
	// should start on Skip.
	Selected int
	// Custom is a stored path that matches none of the choices, typed by hand
	// on an earlier visit.
	Custom string
}

// DirectoryChoicesFor lists the rom folders a platform can map to.
//
// Folders the firmware expects but that are missing come first as offers to
// create them, then the folders already on disk that belong to this platform.
func DirectoryChoicesFor(req ChoiceRequest) Choices {
	tagged := cfw.Lookup(req.CFW).UsesTaggedRomFolders()
	display := func(name string) string {
		if tagged {
			return textmatch.ParseTag(name)
		}
		return name
	}

	expected := cfw.PlatformDirectories(req.CFW, req.Platform.FSSlug, req.PlatformsBinding)
	choices := make([]DirectoryChoice, 0, len(expected)+len(req.Directories))

	for _, dir := range expected {
		if !cfw.IsPlatformDirectory(req.CFW, dir, req.Directories) {
			choices = append(choices, DirectoryChoice{RelativePath: dir, Display: display(dir), Create: true})
		}
	}
	firstCreate := len(choices) > 0

	for _, dir := range req.Directories {
		if cfw.IsPlatformDirectory(req.CFW, dir, expected) {
			choices = append(choices, DirectoryChoice{RelativePath: dir, Display: display(dir)})
		}
	}

	result := Choices{Directories: choices, Selected: -1}

	// A return visit restores what the user chose, even where that no longer
	// matches the folder the name would suggest.
	if len(req.Existing) > 0 {
		mapping := req.Existing[req.Platform.FSSlug]
		if mapping.RelativePath == "" {
			return result
		}
		for i, choice := range choices {
			if choice.RelativePath == mapping.RelativePath {
				result.Selected = i
				return result
			}
		}
		result.Custom = mapping.RelativePath
		return result
	}

	for i, choice := range choices {
		if !choice.Create && cfw.DirectoryMatchesPlatform(req.CFW, req.Platform.FSSlug, choice.RelativePath) {
			result.Selected = i
			return result
		}
	}

	if req.AutoSelect && firstCreate {
		result.Selected = 0
	}
	return result
}

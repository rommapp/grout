package cfw

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"grout/settings"
	"grout/textmatch"
)

// PlatformDirectories returns the rom folder names firmware c accepts for a
// platform.
//
// platformsBinding is the server's own slug mapping, which takes precedence:
// an admin who has told RomM that "ms" means "sms" expects grout to look where
// the server says. A platform the firmware does not know falls back to its own
// slug, so an unmapped platform still gets a sensible folder.
func PlatformDirectories(c CFW, fsSlug string, platformsBinding map[string]string) []string {
	slug := fsSlug
	if bound, ok := platformsBinding[fsSlug]; ok {
		slog.Default().Debug("Using platform binding for CFW lookup", "fsSlug", fsSlug, "boundTo", bound)
		slug = bound
	}

	if dirs, ok := GetPlatformMap(c)[slug]; ok && len(dirs) > 0 {
		return dirs
	}
	return []string{slug}
}

// DirectoriesMatch reports whether two folder names mean the same platform.
//
// MinUI and NextUI allow a tag, so "Game Boy Advance (GBA)" and "GBA" are the
// same folder to them; elsewhere the names must match exactly.
func DirectoriesMatch(c CFW, a, b string) bool {
	if Lookup(c).UsesTaggedRomFolders() {
		return textmatch.ParseTag(a) == textmatch.ParseTag(b)
	}
	return a == b
}

// DirectoryMatchesPlatform reports whether a folder on disk holds this
// platform's roms.
func DirectoryMatchesPlatform(c CFW, platformFSSlug, dirName string) bool {
	slug := FSSlugToFolder(c, platformFSSlug)
	base := Lookup(c).RomFolderBase(dirName, textmatch.ParseTag)

	if Lookup(c).UsesTaggedRomFolders() {
		return textmatch.ParseTag(slug) == base
	}
	return slug == base
}

// IsPlatformDirectory reports whether dirName is one of the folders this
// firmware accepts for a platform.
func IsPlatformDirectory(c CFW, dirName string, platformDirectories []string) bool {
	for _, dir := range platformDirectories {
		if DirectoriesMatch(c, dir, dirName) {
			return true
		}
	}
	return false
}

// CreateRomDirectories makes the folders a set of mappings points at, skipping
// those that already exist.
func CreateRomDirectories(mappings map[string]settings.DirectoryMapping, romDirectory string, existing []string) error {
	present := make(map[string]bool, len(existing))
	for _, name := range existing {
		present[name] = true
	}

	for _, mapping := range mappings {
		if mapping.RelativePath == "" || present[mapping.RelativePath] {
			continue
		}

		path := filepath.Join(romDirectory, mapping.RelativePath)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("creating rom directory %s: %w", path, err)
		}
		slog.Default().Info("Created ROM directory", "path", path)
	}

	return nil
}

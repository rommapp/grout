package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"grout/constants"
	"grout/romm"
)

func ParseTag(input string) string {
	cleaned := filepath.Clean(input)

	tags := constants.TagRegex.FindAllStringSubmatch(cleaned, -1)

	var foundTags []string
	foundTag := ""

	if len(tags) > 0 {
		for _, tagPair := range tags {
			foundTags = append(foundTags, tagPair[0])
		}

		foundTag = strings.Join(foundTags, " ")
	}

	foundTag = strings.ReplaceAll(foundTag, "(", "")
	foundTag = strings.ReplaceAll(foundTag, ")", "")

	return foundTag
}

func nameCleaner(name string, stripTag bool) (string, string) {
	cleaned := filepath.Clean(name)

	tags := constants.TagRegex.FindAllStringSubmatch(cleaned, -1)

	var foundTags []string
	foundTag := ""

	if len(tags) > 0 {
		for _, tagPair := range tags {
			foundTags = append(foundTags, tagPair[0])
		}

		foundTag = strings.Join(foundTags, " ")
	}

	if stripTag {
		for _, tag := range foundTags {
			cleaned = strings.ReplaceAll(cleaned, tag, "")
		}
	}

	orderedFolderRegex := constants.OrderedFolderRegex.FindStringSubmatch(cleaned)

	if len(orderedFolderRegex) > 0 {
		cleaned = strings.ReplaceAll(cleaned, orderedFolderRegex[0], "")
	}

	cleaned = strings.ReplaceAll(cleaned, ":", " -")

	cleaned = strings.TrimSpace(cleaned)

	foundTag = strings.ReplaceAll(foundTag, "(", "")
	foundTag = strings.ReplaceAll(foundTag, ")", "")

	return cleaned, foundTag
}

// PrepareRomNames cleans and sorts ROM names for display.
func PrepareRomNames(games []romm.Rom, config Config) []romm.Rom {
	for i := range games {
		regions := strings.Join(games[i].Regions, ", ")

		cleanedName, _ := nameCleaner(games[i].Name, true)
		games[i].DisplayName = cleanedName

		if len(regions) > 0 {
			dn := fmt.Sprintf("%s (%s)", cleanedName, regions)
			games[i].DisplayName = dn
		}

	}

	slices.SortFunc(games, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return games
}

// IsGameDownloadedLocally checks if a game's ROM file exists locally.
func IsGameDownloadedLocally(game romm.Rom, config Config) bool {
	if game.PlatformSlug == "" {
		return false
	}

	platform := romm.Platform{
		ID:   game.PlatformID,
		Slug: game.PlatformSlug,
		Name: game.PlatformDisplayName,
	}

	romDirectory := GetPlatformRomDirectory(config, platform)

	if game.HasMultipleFiles {
		m3uPath := filepath.Join(romDirectory, game.FsNameNoExt+".m3u")
		if _, err := os.Stat(m3uPath); err == nil {
			return true
		}
	} else if len(game.Files) > 0 {
		romPath := filepath.Join(romDirectory, game.Files[0].FileName)
		if _, err := os.Stat(romPath); err == nil {
			return true
		}
	}

	return false
}

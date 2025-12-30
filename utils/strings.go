package utils

import (
	"fmt"
	"grout/constants"
	"grout/romm"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const Downloaded = "\U000F01DA"

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

func IsGameDownloadedLocally(game romm.Rom, config Config) bool {
	// Need platform info to get ROM directory
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
		m3uPath := filepath.Join(romDirectory, game.DisplayName+".m3u")
		if _, err := os.Stat(m3uPath); err == nil {
			return true
		}
	} else if len(game.Files) > 0 {
		// For single-file ROMs, check if the file exists based on filename
		romPath := filepath.Join(romDirectory, game.Files[0].FileName)
		if _, err := os.Stat(romPath); err == nil {
			return true
		}
	}

	return false
}

func FormatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

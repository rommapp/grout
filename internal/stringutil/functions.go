package stringutil

import (
	"fmt"
	"grout/romm"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var TagRegex = regexp.MustCompile(`\((.*?)\)`)
var OrderedFolderRegex = regexp.MustCompile(`\d+\)\s`)

func StripExtension(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func FormatBytes(bytes int64) string {
	const unit int64 = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func ParseTag(input string) string {
	cleaned := filepath.Clean(input)

	tags := TagRegex.FindAllStringSubmatch(cleaned, -1)

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

	tags := TagRegex.FindAllStringSubmatch(cleaned, -1)

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

	orderedFolderRegex := OrderedFolderRegex.FindStringSubmatch(cleaned)

	if len(orderedFolderRegex) > 0 {
		cleaned = strings.ReplaceAll(cleaned, orderedFolderRegex[0], "")
	}

	cleaned = strings.ReplaceAll(cleaned, ":", " -")

	cleaned = strings.TrimSpace(cleaned)

	foundTag = strings.ReplaceAll(foundTag, "(", "")
	foundTag = strings.ReplaceAll(foundTag, ")", "")

	return cleaned, foundTag
}

func PrepareRomNames(games []romm.Rom) []romm.Rom {
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

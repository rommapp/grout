package utils

import (
	"fmt"
	"grout/constants"
	"grout/romm"
	"path"
	"path/filepath"
	"slices"
	"strings"
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

func NameCleaner(name string, stripTag bool) (string, string) {
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

	cleaned = strings.ReplaceAll(cleaned, path.Ext(cleaned), "")

	cleaned = strings.TrimSpace(cleaned)

	foundTag = strings.ReplaceAll(foundTag, "(", "")
	foundTag = strings.ReplaceAll(foundTag, ")", "")

	return cleaned, foundTag
}

func PrepareRomNames(games []romm.Rom) []romm.Rom {
	for i := range games {
		regions := strings.Join(games[i].Regions, ", ")

		cleanedName, _ := NameCleaner(games[i].Name, true)
		games[i].DisplayName = cleanedName

		if len(regions) > 0 {
			dn := fmt.Sprintf("%s (%s)", cleanedName, regions)
			games[i].DisplayName = dn
		}

		games[i].ListName = games[i].DisplayName
	}

	slices.SortFunc(games, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return games
}

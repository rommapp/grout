package textmatch

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var tagRegex = regexp.MustCompile(`\((.*?)\)`)
var orderedFolderRegex = regexp.MustCompile(`\d+\)\s`)

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

	tags := tagRegex.FindAllStringSubmatch(cleaned, -1)

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

	tags := tagRegex.FindAllStringSubmatch(cleaned, -1)

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

	if prefix := orderedFolderRegex.FindStringSubmatch(cleaned); len(prefix) > 0 {
		cleaned = strings.ReplaceAll(cleaned, prefix[0], "")
	}

	cleaned = strings.ReplaceAll(cleaned, ":", " -")

	// Removing a tag leaves the spaces that surrounded it behind, so
	// "Chrono Trigger (Japan) Special" would otherwise keep a double space.
	for strings.Contains(cleaned, "  ") {
		cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	}

	cleaned = strings.TrimSpace(cleaned)

	foundTag = strings.ReplaceAll(foundTag, "(", "")
	foundTag = strings.ReplaceAll(foundTag, ")", "")

	return cleaned, foundTag
}

func PrepareRomName(name string, regions []string) string {
	r := strings.Join(regions, ", ")

	cleanedName, _ := nameCleaner(name, true)
	displayName := cleanedName

	if len(regions) > 0 {
		dn := fmt.Sprintf("%s (%s)", cleanedName, r)
		displayName = dn
	}

	return displayName
}

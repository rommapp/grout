package stringutil

import (
	"fmt"
	"grout/romm"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var TagRegex = regexp.MustCompile(`\((.*?)\)`)
var OrderedFolderRegex = regexp.MustCompile(`\d+\)\s`)
var BracketRegex = regexp.MustCompile(`\[.*?]`)

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
		games[i].DisplayName = PrepareRomName(games[i].Name, games[i].Regions)
	}

	slices.SortFunc(games, func(a, b romm.Rom) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return games
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

// stripDiacritics removes accents and diacritical marks from a string.
// e.g., "Pokémon" -> "Pokemon"
func stripDiacritics(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	return result
}

func NormalizeForComparison(name string) string {
	name = StripExtension(name)

	name, _ = nameCleaner(name, true)
	name = BracketRegex.ReplaceAllString(name, "")

	name = strings.ToLower(strings.TrimSpace(name))

	// Strip diacritics so "Pokémon" matches "Pokemon"
	name = stripDiacritics(name)

	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, ".", " ")

	for strings.Contains(name, "  ") {
		name = strings.ReplaceAll(name, "  ", " ")
	}

	return strings.TrimSpace(name)
}

func LevenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	r1 := []rune(s1)
	r2 := []rune(s2)

	rows := len(r1) + 1
	cols := len(r2) + 1
	matrix := make([][]int, rows)
	for i := range matrix {
		matrix[i] = make([]int, cols)
	}

	for i := 0; i < rows; i++ {
		matrix[i][0] = i
	}
	for j := 0; j < cols; j++ {
		matrix[0][j] = j
	}

	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			cost := 1
			if r1[i-1] == r2[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,
				matrix[i][j-1]+1,
				matrix[i-1][j-1]+cost,
			)
		}
	}

	return matrix[rows-1][cols-1]
}

func Similarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	maxLen := max(len(s1), len(s2))
	if maxLen == 0 {
		return 1.0
	}

	distance := LevenshteinDistance(s1, s2)
	return 1.0 - float64(distance)/float64(maxLen)
}

// PrefixMatchSimilarity checks if the shorter string is a prefix of the longer
// string at a word boundary. Returns 0.95 if matched (high confidence but not
// exact), 0.0 if not. This handles cases like "Pokemon Red Nuzlocke" matching
// "Pokemon Red" where the user added a custom suffix.
func PrefixMatchSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	var shorter, longer string
	if len(s1) <= len(s2) {
		shorter, longer = s1, s2
	} else {
		shorter, longer = s2, s1
	}

	if shorter == "" {
		return 0.0
	}

	// Check if longer starts with shorter
	if !strings.HasPrefix(longer, shorter) {
		return 0.0
	}

	// Ensure it's at a word boundary (next char is space or end of string)
	if len(longer) == len(shorter) {
		return 1.0 // Exact match
	}

	nextChar := longer[len(shorter)]
	if nextChar == ' ' {
		return 0.95 // High confidence prefix match at word boundary
	}

	return 0.0
}

// CommonPrefixSimilarity checks if two strings share a common word prefix.
// This handles cases like "pokemon red nuzlocke" matching "pokemon red version"
// where both share "pokemon red" as a common prefix.
// Returns 0.85 if at least 2 words match from the start, indicating high confidence.
func CommonPrefixSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}

	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Count matching words from the start
	matchingWords := 0
	minWords := min(len(words1), len(words2))
	for i := 0; i < minWords; i++ {
		if words1[i] == words2[i] {
			matchingWords++
		} else {
			break
		}
	}

	// Require at least 2 matching words to be considered a match
	// This ensures "Pokemon Red Nuzlocke" matches "Pokemon Red Version"
	// but "Pokemon Nuzlocke" won't match "Pokemon Stadium"
	if matchingWords >= 2 {
		return 0.85
	}

	return 0.0
}

// BestSimilarity returns the highest similarity score between two strings,
// using Levenshtein similarity, prefix matching, and common prefix matching.
func BestSimilarity(s1, s2 string) float64 {
	levenshtein := Similarity(s1, s2)
	prefix := PrefixMatchSimilarity(s1, s2)
	commonPrefix := CommonPrefixSimilarity(s1, s2)
	return max(levenshtein, prefix, commonPrefix)
}

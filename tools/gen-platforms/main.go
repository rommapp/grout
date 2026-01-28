// gen-platforms reads markdown platform tables from docs/platforms/ and generates
// the corresponding platforms.json files in cfw/*/data/.
//
// Usage: go run tools/gen-platforms/main.go [cfw-name]
//
// If no cfw-name is provided, all platforms are generated.
// Example: go run tools/gen-platforms/main.go knulli
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var cfwMapping = map[string]string{
	"KNULLI":  "knulli",
	"MUOS":    "muos",
	"NEXTUI":  "nextui",
	"ROCKNIX": "rocknix",
	"SPRUCE":  "spruce",
}

func main() {
	var targets []string

	if len(os.Args) > 1 {
		arg := strings.ToUpper(os.Args[1])
		if _, ok := cfwMapping[arg]; !ok {
			fmt.Fprintf(os.Stderr, "Unknown CFW: %s\n", os.Args[1])
			fmt.Fprintf(os.Stderr, "Valid options: knulli, muos, nextui, rocknix, spruce\n")
			os.Exit(1)
		}
		targets = []string{arg}
	} else {
		for k := range cfwMapping {
			targets = append(targets, k)
		}
	}

	for _, cfw := range targets {
		if err := generatePlatforms(cfw); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", cfw, err)
			os.Exit(1)
		}
	}
}

func generatePlatforms(cfwName string) error {
	mdPath := filepath.Join("docs", "platforms", cfwName+".md")
	jsonPath := filepath.Join("cfw", cfwMapping[cfwName], "data", "platforms.json")

	file, err := os.Open(mdPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", mdPath, err)
	}
	defer file.Close()

	platforms := make(map[string][]string)
	scanner := bufio.NewScanner(file)

	// Regex to match table rows: | col1 | col2 | col3 |
	tableRowRe := regexp.MustCompile(`^\|\s*(.+?)\s*\|\s*(.+?)\s*\|\s*(.+?)\s*\|$`)
	// Regex to match separator row: |---|---|---|
	separatorRe := regexp.MustCompile(`^\|[-:\s|]+\|$`)

	inTable := false
	skipNextSeparator := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Detect table header
		if strings.Contains(line, "| Platform Name") && strings.Contains(line, "| RomM Fs Slug") {
			inTable = true
			skipNextSeparator = true
			continue
		}

		// Skip separator rows
		if separatorRe.MatchString(line) {
			if skipNextSeparator {
				skipNextSeparator = false
			}
			continue
		}

		// End of table
		if inTable && !strings.HasPrefix(line, "|") {
			inTable = false
			continue
		}

		// Parse table rows (3 columns: Platform Name | RomM Fs Slug | Folder(s))
		if inTable {
			matches := tableRowRe.FindStringSubmatch(line)
			if len(matches) == 4 {
				slug := strings.TrimSpace(matches[2])
				foldersRaw := strings.TrimSpace(matches[3])
				folders := parseFolders(foldersRaw)
				platforms[slug] = folders
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading %s: %w", mdPath, err)
	}

	// Write JSON output
	jsonData, err := json.MarshalIndent(platforms, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", jsonPath, err)
	}

	fmt.Printf("Generated %s (%d platforms)\n", jsonPath, len(platforms))
	return nil
}

func parseFolders(raw string) []string {
	// Handle *(none)* as empty array
	if raw == "*(none)*" || raw == "" {
		return []string{}
	}

	// Split by comma and trim whitespace
	parts := strings.Split(raw, ",")
	var folders []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			folders = append(folders, p)
		}
	}
	return folders
}

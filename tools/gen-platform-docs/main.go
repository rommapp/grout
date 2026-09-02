// gen-platform-docs generates the platform mapping tables in docs/platforms/*.md
// from the platforms.json files the application reads.
//
// The direction matters: platforms.json is embedded in the binary and decides
// where roms are written on device, so the docs are a rendering of it. Generated
// the other way, a typo in a markdown table silently changes runtime behaviour.
//
// Usage:
//
//	go run ./tools/gen-platform-docs           # rewrite every doc
//	go run ./tools/gen-platform-docs muos      # rewrite one doc
//	go run ./tools/gen-platform-docs -check    # report drift, write nothing
//
// -check is for CI: it exits non-zero if any doc is out of date.
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
)

//go:embed platform_names.json
var platformNamesJSON []byte

// cfwDocs lists the firmwares with a mapping page.
var cfwDocs = []string{
	"allium", "arkos", "batocera", "knulli", "koriki", "minui",
	"muos", "nextui", "onion", "rocknix", "spruce", "trimui",
}

const (
	noneMarker  = "*(none)*"
	headerFirst = "| Platform Name"
	headerSlug  = "| RomM Fs Slug"
)

var separatorRe = regexp.MustCompile(`^\|[-:\s|]+\|$`)

func main() {
	check := flag.Bool("check", false, "report out-of-date docs and exit non-zero; write nothing")
	flag.Parse()

	names, err := loadPlatformNames()
	if err != nil {
		fail(err)
	}

	targets := cfwDocs
	if args := flag.Args(); len(args) > 0 {
		targets = nil
		for _, a := range args {
			a = strings.ToLower(a)
			if !known(a) {
				fail(fmt.Errorf("unknown cfw %q; valid: %s", a, strings.Join(cfwDocs, ", ")))
			}
			targets = append(targets, a)
		}
	}

	var stale []string
	for _, cfw := range targets {
		changed, err := generate(cfw, names, *check)
		if err != nil {
			fail(err)
		}
		if changed {
			stale = append(stale, cfw)
		}
	}

	if *check {
		if len(stale) > 0 {
			fmt.Fprintf(os.Stderr, "platform docs are out of date: %s\n", strings.Join(stale, ", "))
			fmt.Fprintln(os.Stderr, "run: go run ./tools/gen-platform-docs")
			os.Exit(1)
		}
		fmt.Println("platform docs are up to date")
		return
	}

	if len(stale) == 0 {
		fmt.Println("platform docs already up to date")
		return
	}
	fmt.Printf("updated: %s\n", strings.Join(stale, ", "))
}

func known(cfw string) bool {
	return slices.Contains(cfwDocs, cfw)
}

func loadPlatformNames() (map[string]string, error) {
	var names map[string]string
	if err := json.Unmarshal(platformNamesJSON, &names); err != nil {
		return nil, fmt.Errorf("parse platform_names.json: %w", err)
	}
	return names, nil
}

// generate rewrites the mapping table in one doc, reporting whether it
// changed. Writes nothing when dryRun is set.
func generate(cfw string, names map[string]string, dryRun bool) (bool, error) {
	jsonPath := filepath.Join("cfw", cfw, "data", "platforms.json")
	docPath := filepath.Join("docs", "platforms", cfw+".md")

	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", jsonPath, err)
	}
	var platforms map[string][]string
	if err := json.Unmarshal(raw, &platforms); err != nil {
		return false, fmt.Errorf("parse %s: %w", jsonPath, err)
	}

	doc, err := os.ReadFile(docPath)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", docPath, err)
	}

	// Keep the line ending style the file already uses.
	newline := "\n"
	if strings.Contains(string(doc), "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(doc), "\r\n", "\n"), "\n")

	start, end, err := findTable(lines)
	if err != nil {
		return false, fmt.Errorf("%s: %w", docPath, err)
	}

	// Reuse the existing header and separator so column widths, and therefore
	// the rest of the file, stay byte-identical.
	header, separator := lines[start], lines[start+1]
	widths, err := columnWidths(separator)
	if err != nil {
		return false, fmt.Errorf("%s: %w", docPath, err)
	}

	slugs := make([]string, 0, len(platforms))
	for slug := range platforms {
		slugs = append(slugs, slug)
	}
	var missing []string
	for _, slug := range slugs {
		if _, ok := names[slug]; !ok {
			missing = append(missing, slug)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return false, fmt.Errorf("%s: no display name for %s; add them to tools/gen-platform-docs/platform_names.json",
			jsonPath, strings.Join(missing, ", "))
	}

	// Order by display name.
	sort.Slice(slugs, func(i, j int) bool {
		ni, nj := names[slugs[i]], names[slugs[j]]
		if ni != nj {
			return naturalLess(ni, nj)
		}
		return slugs[i] < slugs[j]
	})

	table := make([]string, 0, len(slugs)+2)
	table = append(table, header, separator)
	for _, slug := range slugs {
		folders := noneMarker
		if len(platforms[slug]) > 0 {
			folders = strings.Join(platforms[slug], ", ")
		}
		table = append(table, row(widths, names[slug], slug, folders))
	}

	updated := make([]string, 0, len(lines)-(end-start+1)+len(table))
	updated = append(updated, lines[:start]...)
	updated = append(updated, table...)
	updated = append(updated, lines[end+1:]...)

	out := strings.Join(updated, newline)
	if out == string(doc) {
		return false, nil
	}
	if dryRun {
		return true, nil
	}
	if err := os.WriteFile(docPath, []byte(out), 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", docPath, err)
	}
	return true, nil
}

// findTable returns the inclusive line range of the mapping table.
func findTable(lines []string) (start, end int, err error) {
	start = -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, headerFirst) && strings.Contains(t, headerSlug) {
			start = i
			break
		}
	}
	if start == -1 {
		return 0, 0, fmt.Errorf("no %q table header found", headerFirst)
	}
	if start+1 >= len(lines) || !separatorRe.MatchString(strings.TrimSpace(lines[start+1])) {
		return 0, 0, fmt.Errorf("table header at line %d is not followed by a separator row", start+1)
	}

	end = start + 1
	for end+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[end+1]), "|") {
		end++
	}
	return start, end, nil
}

// naturalLess compares digit runs numerically, so "Atari 800" precedes
// "Atari 2600" rather than sorting after "Atari 7800".
func naturalLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ad, bd := isDigit(a[ai]), isDigit(b[bi])
		if ad && bd {
			aStart, bStart := ai, bi
			for ai < len(a) && isDigit(a[ai]) {
				ai++
			}
			for bi < len(b) && isDigit(b[bi]) {
				bi++
			}
			// Shorter run is smaller once leading zeroes are ignored.
			an := strings.TrimLeft(a[aStart:ai], "0")
			bn := strings.TrimLeft(b[bStart:bi], "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			continue
		}
		ac, bc := lower(a[ai]), lower(b[bi])
		if ac != bc {
			return ac < bc
		}
		ai++
		bi++
	}
	return len(a)-ai < len(b)-bi
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func columnWidths(separator string) ([]int, error) {
	cells := strings.Split(strings.TrimSpace(separator), "|")
	if len(cells) < 5 {
		return nil, fmt.Errorf("separator row has %d columns, want 3", len(cells)-2)
	}
	return []int{len(cells[1]), len(cells[2]), len(cells[3])}, nil
}

// row renders one table row, padding cells to their column width. Overflowing
// content keeps a single trailing space before the closing pipe.
func row(widths []int, cells ...string) string {
	var b strings.Builder
	b.WriteString("|")
	for i, c := range cells {
		cell := " " + c
		if len(cell) < widths[i] {
			cell += strings.Repeat(" ", widths[i]-len(cell))
		} else {
			cell += " "
		}
		b.WriteString(cell)
		b.WriteString("|")
	}
	return b.String()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gen-platform-docs:", err)
	os.Exit(1)
}

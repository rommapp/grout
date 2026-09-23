// archcheck enforces grout's package layering.
//
// Go rejects an import cycle but says nothing about direction, so a device
// package importing the HTTP client compiles fine. archcheck states the
// direction and fails the build when an import goes the wrong way.
//
// It runs on `go list` rather than a type-checking linter: grout needs cgo and
// SDL2 to compile, but `go list` only resolves the import graph, so this runs
// on a bare CI machine with CGO_ENABLED=0.
//
// Three rules:
//
//   - layer:   an import must be permitted by the matrix in layers.go.
//   - toolkit: only ui and cmd may import the gabagool UI toolkit. Anything
//     else linking SDL needs a display to run its tests.
//   - global:  only cmd may hold package-level mutable state.
//
// Known violations live in allow.txt, so the rules can be turned on before the
// code satisfies them. archcheck fails both when a new violation appears and
// when an allowlisted one is fixed but left behind, so the list only shrinks.
//
// Usage:
//
//	go run ./tools/archcheck            # check, exit non-zero on failure
//	go run ./tools/archcheck -update    # rewrite allow.txt from the current tree
//	go run ./tools/archcheck -v         # also print the clean summary
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const allowFile = "tools/archcheck/allow.txt"

func main() {
	update := flag.Bool("update", false, "rewrite allow.txt from the current tree")
	verbose := flag.Bool("v", false, "print a summary even when everything passes")
	flag.Parse()

	pkgs, err := loadPackages()
	if err != nil {
		fail(err)
	}

	violations := check(pkgs)

	if *update {
		if err := writeAllowFile(violations); err != nil {
			fail(err)
		}
		fmt.Printf("wrote %s with %d entries\n", allowFile, len(violations))
		return
	}

	allowed, err := readAllowFile()
	if err != nil {
		fail(err)
	}

	var added []violation
	found := make(map[string]bool, len(violations))
	for _, v := range violations {
		found[v.Key()] = true
		if !allowed[v.Key()] {
			added = append(added, v)
		}
	}

	var stale []string
	for key := range allowed {
		if !found[key] {
			stale = append(stale, key)
		}
	}
	sort.Strings(stale)

	if len(added) == 0 && len(stale) == 0 {
		if *verbose {
			fmt.Printf("archcheck: %d packages, %d known violations remaining\n", len(pkgs), len(violations))
		}
		return
	}

	if len(added) > 0 {
		fmt.Fprintf(os.Stderr, "archcheck: %d new violation(s)\n\n", len(added))
		for _, v := range added {
			fmt.Fprintf(os.Stderr, "  %s\n", v.Detail)
			for _, f := range v.Files {
				fmt.Fprintf(os.Stderr, "      %s\n", f)
			}
		}
		fmt.Fprintf(os.Stderr, "\nEither fix the import, or if it is a deliberate step add it to %s.\n", allowFile)
	}

	if len(stale) > 0 {
		fmt.Fprintf(os.Stderr, "\narchcheck: %d allowlisted violation(s) no longer present\n\n", len(stale))
		for _, key := range stale {
			fmt.Fprintf(os.Stderr, "  %s\n", key)
		}
		fmt.Fprintf(os.Stderr, "\nThese are fixed. Remove them from %s so the list keeps ratcheting down.\n", allowFile)
	}

	os.Exit(1)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "archcheck:", err)
	os.Exit(2)
}

// --- allow file ---

func readAllowFile() (map[string]bool, error) {
	f, err := os.Open(allowFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, err
	}
	defer f.Close()

	allowed := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		allowed[line] = true
	}
	return allowed, scanner.Err()
}

func writeAllowFile(violations []violation) error {
	byRule := map[string][]string{}
	for _, v := range violations {
		byRule[v.Rule] = append(byRule[v.Rule], v.Key())
	}

	var b strings.Builder
	b.WriteString(`# Known layering violations, tolerated until the refactor removes them.
#
# archcheck fails when a violation appears that is not listed here, and also
# when an entry here no longer applies, so this file can only shrink and a fix
# cannot silently leave dead weight behind.
#
# Regenerate with: go run ./tools/archcheck -update
# Do not add entries by hand without a reason; each line is a known defect.
`)

	for _, rule := range []struct{ name, note string }{
		{"layer", "An import that goes against the layer matrix in layers.go."},
		{"toolkit", "Packages outside ui and cmd that import the gabagool UI toolkit,\n# which is why running their tests needs a display."},
		{"global", "Package-level mutable state outside cmd."},
	} {
		keys := byRule[rule.name]
		if len(keys) == 0 {
			continue
		}
		sort.Strings(keys)
		fmt.Fprintf(&b, "\n# %s\n", rule.note)
		for _, k := range keys {
			b.WriteString(k + "\n")
		}
	}

	if err := os.MkdirAll(filepath.Dir(allowFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(allowFile, []byte(b.String()), 0644)
}

// --- package loading ---

type pkg struct {
	ImportPath string
	Dir        string
	Imports    []string
	GoFiles    []string
}

func loadPackages() ([]pkg, error) {
	const format = `{{.ImportPath}}` + sep + `{{.Dir}}` + sep +
		`{{join .Imports ","}}` + sep + `{{join .GoFiles ","}}`

	cmd := exec.Command("go", "list", "-f", format, "./...")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("go list: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("go list: %w", err)
	}

	var pkgs []pkg
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Split(line, sep)
		if len(parts) != 4 {
			continue
		}
		p := pkg{ImportPath: parts[0], Dir: parts[1]}
		if parts[2] != "" {
			p.Imports = strings.Split(parts[2], ",")
		}
		if parts[3] != "" {
			p.GoFiles = strings.Split(parts[3], ",")
		}
		if isExempt(p.ImportPath) {
			continue
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

const sep = "\x1f"

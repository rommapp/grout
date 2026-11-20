package api

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var contractHeading = regexp.MustCompile("(?m)^### `([a-z_.]+)`$")

var contractErrorRow = regexp.MustCompile("^\\| `([a-z_]+)` \\|")

func documentedErrorCodes(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var codes []string
	inSection := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			inSection = line == "## Error codes"
			continue
		}
		if !inSection {
			continue
		}
		if m := contractErrorRow.FindStringSubmatch(line); m != nil {
			codes = append(codes, m[1])
		}
	}
	return codes, nil
}

func checkErrorCodes(t *testing.T, path string) {
	t.Helper()
	documented, err := documentedErrorCodes(path)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, c := range Codes() {
		want = append(want, string(c))
	}
	slices.Sort(want)
	slices.Sort(documented)
	if !slices.Equal(want, documented) {
		t.Fatalf("contract.md error codes %v do not match Codes() %v", documented, want)
	}
}

func TestContractDocumentsEveryErrorCode(t *testing.T) {
	checkErrorCodes(t, "../docs/library/contract.md")
}

func TestContractErrorCodeCheckDetectsMissingRow(t *testing.T) {
	data, err := os.ReadFile("../docs/library/contract.md")
	if err != nil {
		t.Fatal(err)
	}
	var kept []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "| `rate_limited` |") {
			kept = append(kept, line)
		}
	}
	path := filepath.Join(t.TempDir(), "contract.md")
	if err := os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	documented, err := documentedErrorCodes(path)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(documented, "rate_limited") || len(documented) != len(Codes())-1 {
		t.Fatalf("unexpected documented codes %v", documented)
	}
}

func TestContractDocumentsEveryMethod(t *testing.T) {
	data, err := os.ReadFile("../docs/library/contract.md")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, m := range contractHeading.FindAllStringSubmatch(string(data), -1) {
		documented = append(documented, m[1])
	}
	want := slices.Clone(Methods())
	slices.Sort(want)
	slices.Sort(documented)
	if !slices.Equal(want, documented) {
		t.Fatalf("contract.md headings %v do not match Methods() %v", documented, want)
	}
}

package api

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

var contractHeading = regexp.MustCompile("(?m)^### `([a-z_.]+)`$")

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

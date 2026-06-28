package ui

import (
	"slices"
	"testing"

	"grout/romm"
)

// The platform-mapping filters (Category/Family) are built from the distinct values
// present across platforms; when RomM doesn't populate a field, the list is empty and
// the filter row must be hidden instead of showing a useless "All"-only picker (#247).
func TestDistinctPlatformValues(t *testing.T) {
	platforms := []romm.Platform{
		{Category: "console", Family: "Nintendo"},
		{Category: "handheld", Family: "Nintendo"},
		{Category: "console", Family: "Sega"}, // duplicate category
		{Category: "", Family: ""},            // empty values are skipped
	}

	gotCategory := distinctPlatformValues(platforms, func(p romm.Platform) string { return p.Category })
	if want := []string{"console", "handheld"}; !slices.Equal(gotCategory, want) {
		t.Errorf("category values = %v, want %v (sorted, deduped, no empties)", gotCategory, want)
	}

	gotFamily := distinctPlatformValues(platforms, func(p romm.Platform) string { return p.Family })
	if want := []string{"Nintendo", "Sega"}; !slices.Equal(gotFamily, want) {
		t.Errorf("family values = %v, want %v", gotFamily, want)
	}
}

func TestDistinctPlatformValues_AllEmptyHidesFilter(t *testing.T) {
	// RomM returned no category/family metadata for any platform: the result is empty,
	// which is the signal to hide the filter entirely.
	platforms := []romm.Platform{
		{Category: "", Family: "", Generation: 3},
		{Category: "", Family: "", Generation: 4},
	}

	if got := distinctPlatformValues(platforms, func(p romm.Platform) string { return p.Category }); len(got) != 0 {
		t.Errorf("expected no category values (filter hidden), got %v", got)
	}
	if got := distinctPlatformValues(platforms, func(p romm.Platform) string { return p.Family }); len(got) != 0 {
		t.Errorf("expected no family values (filter hidden), got %v", got)
	}
}

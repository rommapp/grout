package catalog

import (
	"slices"
	"testing"

	"grout/romm"
	"grout/settings"
)

func platforms(names ...string) []romm.Platform {
	out := make([]romm.Platform, 0, len(names))
	for _, n := range names {
		out = append(out, romm.Platform{FSSlug: n, Name: n})
	}
	return out
}

func names(platforms []romm.Platform) []string {
	out := make([]string, 0, len(platforms))
	for _, p := range platforms {
		out = append(out, p.Name)
	}
	return out
}

func TestSortAlphabetically(t *testing.T) {
	got := names(SortAlphabetically(platforms("Nintendo 64", "Amiga", "ZX Spectrum", "Game Boy")))
	want := []string{"Amiga", "Game Boy", "Nintendo 64", "ZX Spectrum"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Sorting must not disturb the caller's slice, which the platform list screen
// still holds in its original order.
func TestSortAlphabetically_DoesNotMutateInput(t *testing.T) {
	input := platforms("Nintendo 64", "Amiga")
	SortAlphabetically(input)
	if input[0].Name != "Nintendo 64" {
		t.Errorf("input was reordered: %v", names(input))
	}
}

// The comparison is case sensitive, so an uppercase name sorts before a
// lowercase one. Recorded because it is a real ordering rule, not because it
// is desirable.
func TestSortAlphabetically_IsCaseSensitive(t *testing.T) {
	got := names(SortAlphabetically(platforms("amiga", "ZX Spectrum")))
	want := []string{"ZX Spectrum", "amiga"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSortByOrder(t *testing.T) {
	all := platforms("Amiga", "Game Boy", "Nintendo 64", "ZX Spectrum")

	got := names(SortByOrder(all, []string{"Nintendo 64", "Amiga"}))
	want := []string{"Nintendo 64", "Amiga", "Game Boy", "ZX Spectrum"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v (ordered first, then the rest alphabetically)", got, want)
	}
}

func TestSortByOrder_EmptyOrderSortsAlphabetically(t *testing.T) {
	got := names(SortByOrder(platforms("Nintendo 64", "Amiga"), nil))
	want := []string{"Amiga", "Nintendo 64"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A saved order outlives the platforms it names: a mapping can be removed, or a
// server can stop reporting a platform. Unknown entries must be skipped rather
// than producing a zero-valued platform.
func TestSortByOrder_IgnoresUnknownEntries(t *testing.T) {
	got := names(SortByOrder(platforms("Amiga", "Game Boy"), []string{"Nintendo 64", "Game Boy"}))
	want := []string{"Game Boy", "Amiga"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// A platform added since the order was saved must still appear.
func TestSortByOrder_AppendsNewPlatforms(t *testing.T) {
	got := names(SortByOrder(platforms("Amiga", "Game Boy", "Nintendo 64"), []string{"Nintendo 64"}))
	want := []string{"Nintendo 64", "Amiga", "Game Boy"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPruneOrder(t *testing.T) {
	mappings := map[string]settings.DirectoryMapping{
		"gba": {}, "snes": {},
	}

	got := PruneOrder([]string{"gba", "psx", "snes"}, mappings)
	want := []string{"gba", "snes"}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPruneOrder_EmptyOrderIsUnchanged(t *testing.T) {
	if got := PruneOrder(nil, map[string]settings.DirectoryMapping{"gba": {}}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// Pruning everything must yield an empty order, not a nil that reads as "no
// order saved" and silently reverts to alphabetical.
func TestPruneOrder_AllRemovedYieldsEmptyNotNil(t *testing.T) {
	got := PruneOrder([]string{"gba", "psx"}, map[string]settings.DirectoryMapping{})
	if got == nil {
		t.Fatal("got nil, want an empty slice")
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

package main

import (
	"sort"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
		why  string
	}{
		{"Atari 800", "Atari 2600", true, "digit runs compare by value, not lexically"},
		{"Atari 2600", "Atari 800", false, "reverse of the above"},
		{"Atari 2600", "Atari 5200", true, "same digit length compares digit by digit"},
		{"Atari 7800", "Atari Jaguar", true, "digits sort before letters"},
		{"Commodore 64", "Commodore 128", true, "64 is less than 128"},
		{"Nintendo DS", "Nintendo GameCube", true, "plain alphabetical still holds"},
		{"Nintendo 64", "Nintendo DS", true, "digits before letters at the same position"},
		{"amiga", "Amstrad CPC", true, "comparison is case insensitive"},
		{"Atari 800", "Atari 800", false, "equal values are not less than each other"},
		{"Sega CD", "Sega CD 32X", true, "a prefix sorts before a longer string"},
		{"PICO-8", "PlayStation", true, "letters after the digit run still compare"},
		{"3DO Interactive Multiplayer", "Amiga", true, "leading digits sort first"},
		{"Atari 0800", "Atari 800", false, "leading zeroes are ignored"},
	}

	for _, tt := range tests {
		if got := naturalLess(tt.a, tt.b); got != tt.want {
			t.Errorf("naturalLess(%q, %q) = %v, want %v (%s)", tt.a, tt.b, got, tt.want, tt.why)
		}
	}
}

// The Atari family is the case that motivated natural ordering: a lexical sort
// strands "Atari 800" between "Atari 7800" and "Atari Jaguar".
func TestNaturalLess_SortsAtariFamilyByModelNumber(t *testing.T) {
	got := []string{"Atari Jaguar", "Atari 7800", "Atari 800", "Atari 2600", "Atari 5200", "Atari Lynx"}
	sort.Slice(got, func(i, j int) bool { return naturalLess(got[i], got[j]) })

	want := []string{"Atari 800", "Atari 2600", "Atari 5200", "Atari 7800", "Atari Jaguar", "Atari Lynx"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted order = %v, want %v", got, want)
		}
	}
}

func TestNaturalLess_IsAStrictWeakOrdering(t *testing.T) {
	values := []string{
		"Atari 800", "Atari 2600", "Atari Jaguar", "Amiga", "3DO", "PICO-8",
		"Nintendo 64", "Nintendo DS", "Commodore 128", "Commodore 64", "amiga",
	}
	for _, a := range values {
		for _, b := range values {
			ab, ba := naturalLess(a, b), naturalLess(b, a)
			if ab && ba {
				t.Errorf("naturalLess reports %q < %q and %q < %q", a, b, b, a)
			}
		}
	}
}

func TestColumnWidths(t *testing.T) {
	widths, err := columnWidths("|-------------------------------|----------------------------|---------------------|")
	if err != nil {
		t.Fatalf("columnWidths: %v", err)
	}
	want := []int{31, 28, 21}
	for i := range want {
		if widths[i] != want[i] {
			t.Fatalf("widths = %v, want %v", widths, want)
		}
	}

	if _, err := columnWidths("|---|---|"); err == nil {
		t.Error("expected an error for a two-column separator row")
	}
}

func TestRow(t *testing.T) {
	widths := []int{31, 28, 21}

	got := row(widths, "Atari 800", "atari800", "EIGHTHUNDRED")
	want := "| Atari 800                     | atari800                   | EIGHTHUNDRED        |"
	if got != want {
		t.Errorf("padded row:\n got %q\nwant %q", got, want)
	}

	// Content wider than its column overflows but keeps one trailing space.
	got = row(widths, "Arcade", "arcade", "ARCADE, CPS1, CPS2, CPS3, FBNEO, MAME2003PLUS, NAOMI")
	want = "| Arcade                        | arcade                     | ARCADE, CPS1, CPS2, CPS3, FBNEO, MAME2003PLUS, NAOMI |"
	if got != want {
		t.Errorf("overflowing row:\n got %q\nwant %q", got, want)
	}
}

func TestFindTable(t *testing.T) {
	lines := []string{
		"# Title",
		"",
		"Some prose.",
		"",
		"| Platform Name | RomM Fs Slug | Folder(s) |",
		"|---|---|---|",
		"| Amiga | amiga | AMIGA |",
		"| Arcade | arcade | ARCADE |",
		"",
		`--8<-- "docs/_includes/cfw-links.md"`,
	}

	start, end, err := findTable(lines)
	if err != nil {
		t.Fatalf("findTable: %v", err)
	}
	if start != 4 || end != 7 {
		t.Errorf("start, end = %d, %d; want 4, 7", start, end)
	}

	if _, _, err := findTable([]string{"# Title", "no table here"}); err == nil {
		t.Error("expected an error when the table header is absent")
	}

	if _, _, err := findTable([]string{"| Platform Name | RomM Fs Slug | Folder(s) |", "| Amiga | amiga | AMIGA |"}); err == nil {
		t.Error("expected an error when the separator row is missing")
	}
}

// Every slug in every platforms.json must have a display name, or the generated
// table would show a blank first column.
func TestPlatformNamesCoverEveryPlatform(t *testing.T) {
	names, err := loadPlatformNames()
	if err != nil {
		t.Fatalf("loadPlatformNames: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("platform_names.json is empty")
	}
	for slug, name := range names {
		if name == "" {
			t.Errorf("slug %q has an empty display name", slug)
		}
	}
}

package ui

import (
	"strings"
	"testing"

	"grout/saves"
)

func item(action saves.SyncAction, name string, success bool) saves.SyncItem {
	return saves.SyncItem{
		LocalSave: saves.LocalSave{RomName: name},
		Action:    action,
		Success:   success,
	}
}

// Each section's heading carries a count that the sync itself tallied, while
// the rows under it are picked out by a separate rule here. If the two ever
// disagree the user sees a heading saying three over a list of one.
func TestReportSections_CountsMatchTheRowsListed(t *testing.T) {
	report := saves.SyncReport{
		Uploaded:   2,
		Downloaded: 1,
		Conflicts:  1,
		Errors:     2,
		Items: []saves.SyncItem{
			item(saves.ActionUpload, "Mario", true),
			item(saves.ActionUpload, "Zelda", true),
			item(saves.ActionDownload, "Sonic", true),
			item(saves.ActionConflict, "Metroid", false),
			item(saves.ActionUpload, "Kirby", false),
			item(saves.ActionDownload, "Castlevania", false),
			item(saves.ActionSkip, "Tetris", false),
		},
	}

	sections := reportSections(report)

	if len(sections) != 4 {
		t.Fatalf("got %d sections, want one per outcome that happened", len(sections))
	}

	for _, section := range sections {
		count := countInHeading(t, section.Title)
		if got := len(section.Metadata); got != count {
			t.Errorf("%q lists %d games but its heading says %d", section.Title, got, count)
		}
	}
}

// An outcome that did not happen is not worth a heading.
func TestReportSections_SkipsEmptyOutcomes(t *testing.T) {
	report := saves.SyncReport{
		Uploaded: 1,
		Skipped:  3,
		Items:    []saves.SyncItem{item(saves.ActionUpload, "Mario", true)},
	}

	sections := reportSections(report)

	if len(sections) != 1 {
		t.Fatalf("got %d sections, want only Uploaded", len(sections))
	}
	if !strings.Contains(sections[0].Title, "1") {
		t.Errorf("heading = %q, want the count in it", sections[0].Title)
	}
}

// Nothing to report is its own message, not an empty screen.
func TestReportSections_NothingHappened(t *testing.T) {
	if got := reportSections(saves.SyncReport{Skipped: 5}); len(got) != 0 {
		t.Errorf("got %d sections for a run that changed nothing, want none", len(got))
	}
}

// The server can reject an upload because its own save moved on, and the sync
// resolves that by downloading instead. It is a download to the user, and
// counts and lists as one.
func TestReportSections_SupersededUploadReadsAsADownload(t *testing.T) {
	report := saves.SyncReport{
		Downloaded: 1,
		Items:      []saves.SyncItem{item(saves.ActionDownload, "Mario", true)},
	}

	sections := reportSections(report)

	if len(sections) != 1 || len(sections[0].Metadata) != 1 {
		t.Fatalf("sections = %v, want the game listed once under Downloaded", sections)
	}
	if sections[0].Metadata[0].Label != "Mario" {
		t.Errorf("listed %q, want the game's name", sections[0].Metadata[0].Label)
	}
}

// countInHeading reads the "(n)" a section title ends with.
func countInHeading(t *testing.T, title string) int {
	t.Helper()

	open := strings.LastIndex(title, "(")
	close := strings.LastIndex(title, ")")
	if open < 0 || close < open {
		t.Fatalf("heading %q has no count", title)
	}

	count := 0
	for _, digit := range title[open+1 : close] {
		if digit < '0' || digit > '9' {
			t.Fatalf("heading %q has a non-numeric count", title)
		}
		count = count*10 + int(digit-'0')
	}
	return count
}

package ui

import (
	"testing"

	"grout/cache"
)

// clearLastFilter decides what pressing back removes when a list has been
// narrowed. It undoes the most recent narrowing first, then whatever remains,
// so a person backs out the way they came in.

func withFilters() cache.GameFilter { return cache.GameFilter{Genres: []string{"Puzzle"}} }

func TestClearLastFilter_NothingApplied(t *testing.T) {
	output := GameListOutput{}
	if clearLastFilter(&output, GameListAppliedNone) {
		t.Error("expected false when nothing is applied, so back leaves the screen")
	}
}

// Filters were applied last, so they go first and the search stays.
func TestClearLastFilter_FiltersLastClearsFiltersAndKeepsSearch(t *testing.T) {
	output := GameListOutput{GameFilter: withFilters(), SearchFilter: "sonic"}

	if !clearLastFilter(&output, GameListAppliedFilters) {
		t.Fatal("expected the filters to be cleared")
	}
	if output.GameFilter.HasActiveFilters() {
		t.Error("filters should be gone")
	}
	if output.SearchFilter != "sonic" {
		t.Errorf("search = %q, want it kept", output.SearchFilter)
	}
	if output.LastApplied != GameListAppliedSearch {
		t.Errorf("LastApplied = %v, want the search to become the outstanding narrowing", output.LastApplied)
	}
}

// Search was applied last, so it goes first and the filters stay.
func TestClearLastFilter_SearchLastClearsSearchAndKeepsFilters(t *testing.T) {
	output := GameListOutput{GameFilter: withFilters(), SearchFilter: "sonic"}

	if !clearLastFilter(&output, GameListAppliedSearch) {
		t.Fatal("expected the search to be cleared")
	}
	if output.SearchFilter != "" {
		t.Errorf("search = %q, want it cleared", output.SearchFilter)
	}
	if !output.GameFilter.HasActiveFilters() {
		t.Error("filters should be kept")
	}
	if output.LastApplied != GameListAppliedFilters {
		t.Errorf("LastApplied = %v, want the filters to become the outstanding narrowing", output.LastApplied)
	}
}

// Clearing the only narrowing leaves nothing outstanding, so the next back
// press leaves the screen rather than clearing again.
func TestClearLastFilter_LastOneLeavesNothingApplied(t *testing.T) {
	output := GameListOutput{SearchFilter: "sonic"}
	if !clearLastFilter(&output, GameListAppliedSearch) {
		t.Fatal("expected the search to be cleared")
	}
	if output.LastApplied != GameListAppliedNone {
		t.Errorf("LastApplied = %v, want none", output.LastApplied)
	}
	if clearLastFilter(&output, GameListAppliedNone) {
		t.Error("a second back press should leave the screen")
	}
}

// The recorded "last applied" can be stale, for instance after returning from
// another screen. Whatever is actually set still has to be clearable.
func TestClearLastFilter_StaleLastAppliedStillClears(t *testing.T) {
	output := GameListOutput{GameFilter: withFilters()}

	if !clearLastFilter(&output, GameListAppliedSearch) {
		t.Fatal("expected the filters to be cleared despite a stale LastApplied")
	}
	if output.GameFilter.HasActiveFilters() {
		t.Error("filters should be gone")
	}
}

// Clearing always returns to the top of the list, since the previous position
// pointed into a shorter list.
func TestClearLastFilter_ResetsScrollPosition(t *testing.T) {
	output := GameListOutput{
		SearchFilter:         "sonic",
		LastSelectedIndex:    42,
		LastSelectedPosition: 17,
	}

	clearLastFilter(&output, GameListAppliedSearch)

	if output.LastSelectedIndex != 0 || output.LastSelectedPosition != 0 {
		t.Errorf("position = (%d, %d), want the top of the list",
			output.LastSelectedIndex, output.LastSelectedPosition)
	}
	if output.Action != GameListActionClearSearch {
		t.Errorf("Action = %v, want the screen to redraw", output.Action)
	}
}

// Both narrowings clear one press at a time, never both at once.
func TestClearLastFilter_ClearsOneNarrowingPerPress(t *testing.T) {
	output := GameListOutput{GameFilter: withFilters(), SearchFilter: "sonic"}

	clearLastFilter(&output, GameListAppliedFilters)
	if output.SearchFilter == "" {
		t.Fatal("the first press cleared both narrowings")
	}

	clearLastFilter(&output, output.LastApplied)
	if output.SearchFilter != "" {
		t.Error("the second press should clear the search")
	}
	if output.LastApplied != GameListAppliedNone {
		t.Errorf("LastApplied = %v, want none once both are gone", output.LastApplied)
	}
}

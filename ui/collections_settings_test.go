package ui

import (
	"testing"

	"grout/settings"
)

// Switching a kind of collection on means the cache has never fetched that
// kind, so there is nothing to show until it does. Switching one off needs
// nothing: the cache keeps what it has.
func TestNewlyShownCollections(t *testing.T) {
	tests := []struct {
		name          string
		before, after settings.Config
		want          bool
	}{
		{"nothing changed", settings.Config{}, settings.Config{}, false},
		{
			"regular turned on",
			settings.Config{},
			settings.Config{ShowRegularCollections: true},
			true,
		},
		{
			"smart turned on",
			settings.Config{},
			settings.Config{ShowSmartCollections: true},
			true,
		},
		{
			"virtual turned on",
			settings.Config{},
			settings.Config{ShowVirtualCollections: true},
			true,
		},
		{
			"turned off",
			settings.Config{ShowRegularCollections: true, ShowSmartCollections: true},
			settings.Config{},
			false,
		},
		{
			"one off and another on still needs the new one",
			settings.Config{ShowRegularCollections: true},
			settings.Config{ShowSmartCollections: true},
			true,
		},
		{
			"already on and left on",
			settings.Config{ShowRegularCollections: true},
			settings.Config{ShowRegularCollections: true},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newlyShownCollections(tt.before, tt.after); got != tt.want {
				t.Errorf("newlyShownCollections = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectionsSettings_RoundTrips(t *testing.T) {
	before := settings.Config{
		ShowRegularCollections: true,
		ShowSmartCollections:   false,
		ShowVirtualCollections: true,
		CollectionView:         settings.CollectionViewUnified,
	}
	after := before

	rows := collectionsRows()
	applySettingRows(rows, &after, settingItems(rows, before))

	if after.ShowRegularCollections != before.ShowRegularCollections ||
		after.ShowSmartCollections != before.ShowSmartCollections ||
		after.ShowVirtualCollections != before.ShowVirtualCollections ||
		after.CollectionView != before.CollectionView {
		t.Errorf("opening and saving changed settings:\n before %+v\n after  %+v", before, after)
	}
}

// Reading the screen back and deciding whether a sync is needed must agree:
// a switch the read-back missed would leave the cache without collections it
// is now expected to show.
func TestCollectionsSettings_TurningOneOnAsksForASync(t *testing.T) {
	before := settings.Config{}

	rows := collectionsRows()
	items := settingItems(rows, before)

	// Switch every row to Show, which is the first option.
	for i := range items {
		if rows[i].set != nil && len(items[i].Options) > 0 {
			items[i].SelectedOption = 0
		}
	}

	after := before
	applySettingRows(rows, &after, items)

	if !newlyShownCollections(before, after) {
		t.Error("turning the collections on did not ask for a sync")
	}
}

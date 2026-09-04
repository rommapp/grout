package ui

import (
	"sync/atomic"

	"grout/settings"

	gaba "github.com/BrandonKowalski/gabagool/v2/pkg/gabagool"
)

// settingRow is one row of a settings screen: what it says, and either a value
// it changes or nothing, for a row that only leads somewhere.
//
// The screens differ in what they offer but not in how a row is drawn or read
// back, so that is written here once. A screen that navigates keeps its own
// map from key to destination.
type settingRow struct {
	// key identifies the row when the screen is read back. Matching on the
	// label would break the day two rows read alike in some language.
	key     string
	label   string
	options []gaba.Option
	get     func(settings.Config) any
	set     func(*settings.Config, any)
	// def is what the settings package would fill in. A config it has not
	// reached opens on the same answer the rest of the app uses rather than on
	// whatever is listed first.
	def any
	// visible, when set, hides the row while it holds no meaning.
	visible *atomic.Bool
}

// leadsSomewhere reports whether choosing this row navigates rather than
// changing a value.
func (r settingRow) leadsSomewhere() bool { return r.options == nil }

// clickableRow is a row that only leads somewhere.
func clickableRow(key, labelID, fallback string) settingRow {
	return settingRow{key: key, label: localize(labelID, fallback)}
}

// settingItems renders the rows, each opened on the value the config holds.
func settingItems(rows []settingRow, config settings.Config) []gaba.ItemWithOptions {
	items := make([]gaba.ItemWithOptions, 0, len(rows))

	for _, row := range rows {
		item := gaba.ItemWithOptions{
			Item:        gaba.MenuItem{Text: row.label, Metadata: row.key},
			VisibleWhen: row.visible,
		}
		if row.leadsSomewhere() {
			item.Options = []gaba.Option{{Type: gaba.OptionTypeClickable}}
		} else {
			item.Options = row.options
			item.SelectedOption = optionIndexOr(row.options, row.get(config), row.def)
		}
		items = append(items, item)
	}

	return items
}

// applySettingRows writes back what the user chose. Rows are found by key, so
// a row the screen did not show keeps whatever the config already had.
func applySettingRows(rows []settingRow, config *settings.Config, items []gaba.ItemWithOptions) {
	byKey := make(map[string]settingRow, len(rows))
	for _, row := range rows {
		byKey[row.key] = row
	}

	for _, item := range items {
		key, _ := item.Item.Metadata.(string)
		row, ok := byKey[key]
		if !ok || row.set == nil {
			continue
		}
		if item.SelectedOption < 0 || item.SelectedOption >= len(item.Options) {
			continue
		}
		row.set(config, item.Options[item.SelectedOption].Value)
	}
}

// assign stores a value of the type a row deals in, ignoring anything else, so
// a mistyped option cannot write nonsense into the config.
func assign[T any](set func(*settings.Config, T)) func(*settings.Config, any) {
	return func(config *settings.Config, value any) {
		if typed, ok := value.(T); ok {
			set(config, typed)
		}
	}
}

// trueFalse is the plainest option pair a setting can offer.
func trueFalse() []gaba.Option {
	return []gaba.Option{
		{DisplayName: localize("common_true", "True"), Value: true},
		{DisplayName: localize("common_false", "False"), Value: false},
	}
}

// showHide reads better than true and false for something that is on screen or
// is not.
func showHide() []gaba.Option {
	return []gaba.Option{
		{DisplayName: localize("common_show", "Show"), Value: true},
		{DisplayName: localize("common_hide", "Hide"), Value: false},
	}
}

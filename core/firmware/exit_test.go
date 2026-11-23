package firmware

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/rommapp/grout/core/firmware/host"
)

func exitFW(t *testing.T, body string, reg Registry) *Firmware {
	t.Helper()
	d := mustParse(t, `
id = "x"
[paths]
card = { value = "/x" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
`+body)
	f, err := ResolveWith(d, &host.Fake{}, Overrides{}, reg)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestExitConditionsListed(t *testing.T) {
	want := []ExitCondition{CondGamelistChanged, CondGamesChanged}
	got := ExitConditions()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExitConditions = %v, want %v", got, want)
	}
	if string(CondGamelistChanged) != "gamelist_changed" || string(CondGamesChanged) != "games_changed" {
		t.Fatalf("condition values changed: %q %q", CondGamelistChanged, CondGamesChanged)
	}
}

func TestPendingActionsFiltersAndOrders(t *testing.T) {
	f := exitFW(t, `
[[exit_actions]]
when = "games_changed"
hook = "rescan"

[[exit_actions]]
when = "gamelist_changed"
command = { value = ["restart", "es"] }

[[exit_actions]]
when = "gamelist_changed"
hook = "after"
`, Registry{})
	got := f.PendingActions([]ExitCondition{CondGamelistChanged})
	want := []PendingAction{
		{When: CondGamelistChanged, Command: []string{"restart", "es"}},
		{When: CondGamelistChanged, Hook: "after"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PendingActions = %+v, want %+v", got, want)
	}
	got = f.PendingActions([]ExitCondition{CondGamesChanged, CondGamelistChanged})
	want = append([]PendingAction{{When: CondGamesChanged, Hook: "rescan"}}, want...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PendingActions = %+v, want %+v", got, want)
	}
}

func TestPendingActionsDeduplicates(t *testing.T) {
	f := exitFW(t, `
[[exit_actions]]
when = "gamelist_changed"
command = { value = ["restart", "es"] }

[[exit_actions]]
when = "games_changed"
command = { value = ["restart", "es"] }

[[exit_actions]]
when = "games_changed"
command = { value = ["restart", "es", "now"] }

[[exit_actions]]
when = "gamelist_changed"
hook = "rescan"

[[exit_actions]]
when = "games_changed"
hook = "rescan"
`, Registry{})
	got := f.PendingActions([]ExitCondition{CondGamelistChanged, CondGamesChanged})
	want := []PendingAction{
		{When: CondGamelistChanged, Command: []string{"restart", "es"}},
		{When: CondGamesChanged, Command: []string{"restart", "es", "now"}},
		{When: CondGamelistChanged, Hook: "rescan"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PendingActions = %+v, want %+v", got, want)
	}
}

func TestPendingActionsNoneFired(t *testing.T) {
	f := exitFW(t, `
[[exit_actions]]
when = "gamelist_changed"
command = { value = ["restart"] }
`, Registry{})
	if got := f.PendingActions(nil); got != nil {
		t.Fatalf("nil fired: %+v", got)
	}
	if got := f.PendingActions([]ExitCondition{}); got != nil {
		t.Fatalf("empty fired: %+v", got)
	}
	if got := f.PendingActions([]ExitCondition{CondGamesChanged}); got != nil {
		t.Fatalf("unmatched fired: %+v", got)
	}
}

func TestRunExitHookUnknown(t *testing.T) {
	var ran []string
	reg := Registry{Exit: map[string]ExitHook{
		"known": func(ctx context.Context, h host.Host) error {
			ran = append(ran, "known")
			return nil
		},
	}}
	f := exitFW(t, "", reg)
	if err := f.RunExitHook(context.Background(), "missing"); err == nil {
		t.Fatal("unknown hook ran without error")
	}
	if err := f.RunExitHook(context.Background(), "known"); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ran, []string{"known"}) {
		t.Fatalf("ran %v", ran)
	}
}

func TestExitHooksRegistered(t *testing.T) {
	if Hooks().Exit["leaf_rescan"] == nil {
		t.Fatal("leaf_rescan is not registered")
	}
	merged := MergeHooks(Hooks(), Registry{Exit: map[string]ExitHook{"extra": func(context.Context, host.Host) error { return nil }}})
	if merged.Exit["leaf_rescan"] == nil || merged.Exit["extra"] == nil {
		t.Fatal("MergeHooks dropped exit hooks")
	}
}

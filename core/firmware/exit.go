package firmware

import (
	"context"
	"fmt"
	"slices"

	"github.com/rommapp/grout/core/firmware/host"
)

type ExitCondition string

const (
	CondGamelistChanged ExitCondition = "gamelist_changed"
	CondGamesChanged    ExitCondition = "games_changed"
)

func ExitConditions() []ExitCondition {
	return []ExitCondition{CondGamelistChanged, CondGamesChanged}
}

type ExitHook func(ctx context.Context, h host.Host) error

type ExitAction struct {
	When    ExitCondition `toml:"when"`
	Command *Fact         `toml:"command"`
	Hook    string        `toml:"hook"`
}

type PendingAction struct {
	When    ExitCondition `json:"when"`
	Command []string      `json:"command,omitempty"`
	Hook    string        `json:"hook,omitempty"`
}

func (f *Firmware) PendingActions(fired []ExitCondition) []PendingAction {
	if len(fired) == 0 {
		return nil
	}
	var out []PendingAction
	for _, a := range f.Def.ExitActions {
		when := a.When
		if !slices.Contains(fired, when) {
			continue
		}
		p := PendingAction{When: when, Hook: a.Hook}
		if a.Command != nil {
			picked, ok := a.Command.Pick(f.Version)
			if !ok {
				continue
			}
			p.Command = picked.Strings()
			if len(p.Command) == 0 {
				continue
			}
		}
		if slices.ContainsFunc(out, func(o PendingAction) bool { return sameAction(o, p) }) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func sameAction(a, b PendingAction) bool {
	if a.Hook != "" || b.Hook != "" {
		return a.Hook == b.Hook
	}
	return slices.Equal(a.Command, b.Command)
}

func (f *Firmware) RunExitHook(ctx context.Context, name string) error {
	fn := f.reg.Exit[name]
	if fn == nil {
		return fmt.Errorf("exit hook %q is not registered", name)
	}
	return fn(ctx, f.Host)
}

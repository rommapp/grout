package grout

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
)

const exitDefinition = hostDefinition + `
[[exit_actions]]
when = "gamelist_changed"
command = { value = ["restart-ui", "--now"] }

[[exit_actions]]
when = "games_changed"
hook = "test_rescan"

[[exit_actions]]
when = "games_changed"
command = { value = ["restart-ui", "--now"] }
`

var (
	restartAction = firmware.PendingAction{When: firmware.CondGamelistChanged, Command: []string{"restart-ui", "--now"}}
	rescanAction  = firmware.PendingAction{When: firmware.CondGamesChanged, Hook: "test_rescan"}
)

type hookCalls struct {
	hosts  []host.Host
	err    error
	during func()
}

func exitGrout(t *testing.T, h host.Host, calls *hookCalls) *Grout {
	t.Helper()
	return open(t, Options{
		DataDir:            t.TempDir(),
		FirmwareDefinition: []byte(exitDefinition),
		Host:               h,
		Hooks: firmware.Registry{Exit: map[string]firmware.ExitHook{
			"test_rescan": func(ctx context.Context, h host.Host) error {
				if ctx == nil {
					return errors.New("no context")
				}
				calls.hosts = append(calls.hosts, h)
				if calls.during != nil {
					calls.during()
				}
				return calls.err
			},
		}},
	})
}

func exitHost() *host.Fake {
	return &host.Fake{Env: map[string]string{"ROOT": "/card"}}
}

func fire(t *testing.T, g *Grout, conds ...firmware.ExitCondition) {
	t.Helper()
	if err := g.snapshot().FireConditions(context.Background(), conds...); err != nil {
		t.Fatal(err)
	}
}

func TestExitActionsListAndClear(t *testing.T) {
	ctx := context.Background()
	g := exitGrout(t, exitHost(), &hookCalls{})
	got, err := g.ExitActions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("actions = %#v", got)
	}

	fire(t, g, firmware.CondGamesChanged)
	got, err = g.ExitActions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []firmware.PendingAction{rescanAction, {When: firmware.CondGamesChanged, Command: restartAction.Command}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %+v, want %+v", got, want)
	}

	fire(t, g, firmware.CondGamelistChanged)
	want := []firmware.PendingAction{restartAction, rescanAction}
	for range 2 {
		got, err = g.ExitActions(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("actions = %+v, want %+v", got, want)
		}
	}
	wantFired(t, g, firmware.CondGamelistChanged, firmware.CondGamesChanged)

	got, err = g.ExitActions(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %+v, want %+v", got, want)
	}
	wantFired(t, g, firmware.CondGamesChanged)
	got, err = g.ExitActions(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := []firmware.PendingAction{rescanAction, {When: firmware.CondGamesChanged, Command: restartAction.Command}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("actions after clear = %+v, want %+v", got, want)
	}
}

func TestExitActionsClearKeepsHookCondition(t *testing.T) {
	ctx := context.Background()
	calls := &hookCalls{}
	h := exitHost()
	g := exitGrout(t, h, calls)
	fire(t, g, firmware.CondGamelistChanged, firmware.CondGamesChanged)
	got, err := g.ExitActions(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := []firmware.PendingAction{restartAction, rescanAction}; !reflect.DeepEqual(got, want) {
		t.Fatalf("actions = %+v, want %+v", got, want)
	}
	wantFired(t, g, firmware.CondGamesChanged)
	res, err := g.RunExitActions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls.hosts) != 1 || len(res.Failed) != 0 {
		t.Fatalf("result = %+v, hook calls %d", res, len(calls.hosts))
	}
	wantFired(t, g)
}

func TestRunExitActionsCancelledContext(t *testing.T) {
	calls := &hookCalls{}
	h := exitHost()
	g := exitGrout(t, h, calls)
	fire(t, g, firmware.CondGamelistChanged, firmware.CondGamesChanged)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := g.RunExitActions(ctx)
	var e *Error
	if !errors.As(err, &e) || e.Code != CodeCancelled {
		t.Fatalf("err = %v", err)
	}
	if len(h.Started) != 0 || len(calls.hosts) != 0 {
		t.Fatalf("started %v, hooks %d", h.Started, len(calls.hosts))
	}
	wantFired(t, g, firmware.CondGamelistChanged, firmware.CondGamesChanged)
}

func TestRunExitActionsStartsCommandsWithDelay(t *testing.T) {
	ctx := context.Background()
	h := exitHost()
	g := exitGrout(t, h, &hookCalls{})
	fire(t, g, firmware.CondGamelistChanged)
	res, err := g.RunExitActions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Ran, []firmware.PendingAction{restartAction}) || res.Failed == nil || len(res.Failed) != 0 {
		t.Fatalf("result = %#v", res)
	}
	if want := []host.Started{{Argv: []string{"restart-ui", "--now"}, Delay: 3 * time.Second}}; !reflect.DeepEqual(h.Started, want) {
		t.Fatalf("started = %+v", h.Started)
	}
	if ExitCommandDelay != 3*time.Second {
		t.Fatalf("delay = %s", ExitCommandDelay)
	}
	wantFired(t, g)

	res, err = g.RunExitActions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Ran == nil || res.Failed == nil || len(res.Ran)+len(res.Failed) != 0 || len(h.Started) != 1 {
		t.Fatalf("second run = %#v, started = %+v", res, h.Started)
	}
}

func TestRunExitActionsRunsHook(t *testing.T) {
	h := exitHost()
	calls := &hookCalls{}
	g := exitGrout(t, h, calls)
	fire(t, g, firmware.CondGamesChanged)
	res, err := g.RunExitActions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []firmware.PendingAction{rescanAction, {When: firmware.CondGamesChanged, Command: restartAction.Command}}
	if !reflect.DeepEqual(res.Ran, want) || len(res.Failed) != 0 {
		t.Fatalf("result = %+v", res)
	}
	if len(calls.hosts) != 1 || calls.hosts[0] != host.Host(h) {
		t.Fatalf("hook calls = %+v", calls.hosts)
	}
	wantFired(t, g)
}

func TestRunExitActionsClearsEvenOnFailure(t *testing.T) {
	h := exitHost()
	h.StartErr = errors.New("cannot fork")
	calls := &hookCalls{err: errors.New("socket closed")}
	g := exitGrout(t, h, calls)
	fire(t, g, firmware.CondGamesChanged, firmware.CondGamelistChanged)
	res, err := g.RunExitActions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []ExitActionFailure{
		{Action: restartAction, Message: "cannot fork"},
		{Action: rescanAction, Message: "socket closed"},
	}
	if !reflect.DeepEqual(res.Failed, want) || res.Ran == nil || len(res.Ran) != 0 {
		t.Fatalf("result = %#v", res)
	}
	if len(calls.hosts) != 1 || len(h.Started) != 1 {
		t.Fatalf("not every action attempted: hooks %d, starts %d", len(calls.hosts), len(h.Started))
	}
	wantFired(t, g)
}

func TestRunExitActionsNoRunner(t *testing.T) {
	calls := &hookCalls{}
	g := exitGrout(t, readOnlyHost{exitHost()}, calls)
	fire(t, g, firmware.CondGamelistChanged, firmware.CondGamesChanged)
	res, err := g.RunExitActions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []firmware.PendingAction{rescanAction}; !reflect.DeepEqual(res.Ran, want) {
		t.Fatalf("ran = %+v", res.Ran)
	}
	if want := []ExitActionFailure{{Action: restartAction, Message: "host cannot run commands"}}; !reflect.DeepEqual(res.Failed, want) {
		t.Fatalf("failed = %+v", res.Failed)
	}
	wantFired(t, g)
}

func TestExitActionsAfterClose(t *testing.T) {
	g := exitGrout(t, exitHost(), &hookCalls{})
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := g.ExitActions(context.Background(), false)
	wantCode(t, err, CodeBadRequest)
	_, err = g.RunExitActions(context.Background())
	wantCode(t, err, CodeBadRequest)
}

func TestRunExitActionsKeepsConditionRefiredDuringRun(t *testing.T) {
	for name, tc := range map[string]struct {
		initial []firmware.ExitCondition
		refire  firmware.ExitCondition
	}{
		"own condition":   {[]firmware.ExitCondition{firmware.CondGamesChanged}, firmware.CondGamesChanged},
		"other condition": {[]firmware.ExitCondition{firmware.CondGamelistChanged, firmware.CondGamesChanged}, firmware.CondGamelistChanged},
	} {
		t.Run(name, func(t *testing.T) {
			calls := &hookCalls{}
			g := exitGrout(t, exitHost(), calls)
			calls.during = func() { fire(t, g, tc.refire) }
			fire(t, g, tc.initial...)
			res, err := g.RunExitActions(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(calls.hosts) != 1 || len(res.Failed) != 0 {
				t.Fatalf("result = %+v, hook calls %d", res, len(calls.hosts))
			}
			wantFired(t, g, tc.refire)
		})
	}
}

func TestExitActionsClearKeepsConditionFiredAfterRead(t *testing.T) {
	ctx := context.Background()
	g := exitGrout(t, exitHost(), &hookCalls{})
	fire(t, g, firmware.CondGamesChanged)
	l := g.snapshot()
	read, err := l.FiredConditions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.ExitActions(ctx, true); err != nil {
		t.Fatal(err)
	}
	fire(t, g, firmware.CondGamesChanged)
	if err := l.ClearConditions(ctx, read); err != nil {
		t.Fatal(err)
	}
	wantFired(t, g, firmware.CondGamesChanged)
}

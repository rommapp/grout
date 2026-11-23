package grout

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/firmware/host"
	"github.com/rommapp/grout/core/library"
)

const ExitCommandDelay = 3 * time.Second

var errNoRunner = errors.New("host cannot run commands")

type ExitActionFailure struct {
	Action  firmware.PendingAction `json:"action"`
	Message string                 `json:"message"`
}

type ExitRunResult struct {
	Ran    []firmware.PendingAction `json:"ran"`
	Failed []ExitActionFailure      `json:"failed"`
}

func (g *Grout) ExitActions(ctx context.Context, clear bool) ([]firmware.PendingAction, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer end()
	l := g.snapshot()
	fired, err := l.FiredConditions(ctx)
	if err != nil {
		return nil, classify(ctx, err, CodeStorage)
	}
	actions := g.fw.PendingActions(fired.Conditions)
	if clear {
		clearable := library.Fired{Seq: fired.Seq}
		for _, c := range fired.Conditions {
			if !slices.ContainsFunc(actions, func(a firmware.PendingAction) bool { return a.When == c && a.Hook != "" }) {
				clearable.Conditions = append(clearable.Conditions, c)
			}
		}
		if err := l.ClearConditions(ctx, clearable); err != nil {
			return nil, classify(ctx, err, CodeStorage)
		}
	}
	return list(ctx, actions, nil)
}

func (g *Grout) RunExitActions(ctx context.Context) (ExitRunResult, error) {
	ctx, end, err := g.begin(ctx)
	if err != nil {
		return ExitRunResult{}, err
	}
	defer end()
	if err := ctx.Err(); err != nil {
		return ExitRunResult{}, classify(ctx, err, CodeCancelled)
	}
	l := g.snapshot()
	fired, err := l.FiredConditions(ctx)
	if err != nil {
		return ExitRunResult{}, classify(ctx, err, CodeStorage)
	}
	res := ExitRunResult{Ran: []firmware.PendingAction{}, Failed: []ExitActionFailure{}}
	runner, _ := g.fw.Host.(host.Runner)
	for _, a := range g.fw.PendingActions(fired.Conditions) {
		var err error
		switch {
		case a.Hook != "":
			err = g.fw.RunExitHook(ctx, a.Hook)
		case runner == nil:
			err = errNoRunner
		default:
			err = runner.Start(a.Command, ExitCommandDelay)
		}
		if err != nil {
			g.log.Warn("exit action failed", "when", a.When, "command", a.Command, "hook", a.Hook, "err", err)
			res.Failed = append(res.Failed, ExitActionFailure{Action: a, Message: err.Error()})
			continue
		}
		g.log.Info("exit action started", "when", a.When, "command", a.Command, "hook", a.Hook)
		res.Ran = append(res.Ran, a)
	}
	if err := l.ClearConditions(context.WithoutCancel(ctx), fired); err != nil {
		return res, classify(ctx, err, CodeStorage)
	}
	return res, nil
}

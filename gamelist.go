package grout

import (
	"context"
	"fmt"

	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/core/gamelist"
	"github.com/rommapp/grout/core/library"
)

type GamelistRefreshResult struct {
	Games int `json:"games"`
	gamelist.Result
}

func emptyGamelist() gamelist.Result {
	return gamelist.Result{Written: []string{}, Failed: []gamelist.Failure{}}
}

func (g *Grout) fire(ctx context.Context, l *library.Library, conds ...firmware.ExitCondition) {
	if err := l.FireConditions(context.WithoutCancel(ctx), conds...); err != nil {
		g.log.Warn("saving exit conditions failed", "conditions", conds, "err", err)
	}
}

func (g *Grout) writeEntry(ctx context.Context, l *library.Library, p library.Platform, gameID int, romDir, launch string, folder bool) gamelist.Result {
	if ctx.Err() != nil || g.lists == nil {
		return emptyGamelist()
	}
	meta, err := l.Meta(ctx, gameID)
	if err != nil {
		res := emptyGamelist()
		res.Failed = append(res.Failed, gamelist.Failure{GameID: gameID, Path: launch, Message: err.Error()})
		return res
	}
	cfg, _ := g.settings()
	res := g.lists.Write(ctx, []gamelist.Game{{Meta: meta, Platform: p, RomDir: romDir, LaunchPath: launch, Folder: folder}}, cfg.GamelistOmitsRegion)
	g.logGamelist(res)
	if len(res.Written) > 0 {
		g.fire(ctx, l, firmware.CondGamelistChanged)
	}
	return res
}

func (g *Grout) logGamelist(res gamelist.Result) {
	for _, f := range res.Failed {
		g.log.Warn("gamelist entry not written", "game", f.GameID, "path", f.Path, "err", f.Message)
	}
}

func (g *Grout) GamelistRefresh(ctx context.Context, platformID int) (GamelistRefreshResult, error) {
	ctx, end, err := g.beginFetch(ctx, "gamelist")
	if err != nil {
		return GamelistRefreshResult{}, err
	}
	defer end()
	if g.lists == nil {
		return GamelistRefreshResult{}, newError(CodeInternal, errReadOnly)
	}
	l := g.snapshot()
	all, err := l.AllPlatforms(ctx)
	if err != nil {
		return GamelistRefreshResult{}, classify(ctx, err, CodeStorage)
	}
	platforms := map[int]library.Platform{}
	for _, p := range all {
		platforms[p.ID] = p
	}
	if _, ok := platforms[platformID]; platformID != 0 && !ok {
		return GamelistRefreshResult{}, newError(CodeBadRequest, fmt.Errorf("unknown platform %d", platformID))
	}
	onCard, err := l.OnCard(ctx, platformID)
	if err != nil {
		return GamelistRefreshResult{}, classify(ctx, err, CodeStorage)
	}
	res := GamelistRefreshResult{Games: len(onCard), Result: emptyGamelist()}
	games := make([]gamelist.Game, 0, len(onCard))
	for _, c := range onCard {
		meta, err := l.Meta(ctx, c.GameID)
		if err != nil {
			if ctx.Err() != nil {
				return res, finish(ctx, nil)
			}
			res.Failed = append(res.Failed, gamelist.Failure{GameID: c.GameID, Path: c.LaunchPath, Message: err.Error()})
			continue
		}
		p, ok := platforms[c.PlatformID]
		if !ok {
			p = library.Platform{ID: c.PlatformID}
		}
		dir, _ := artTarget(p.RomDirs, c.LaunchPath, c.Folder)
		games = append(games, gamelist.Game{Meta: meta, Platform: p, RomDir: dir, LaunchPath: c.LaunchPath, Folder: c.Folder})
	}
	cfg, _ := g.settings()
	written := g.lists.Write(ctx, games, cfg.GamelistOmitsRegion)
	res.Written = append(res.Written, written.Written...)
	res.Failed = append(res.Failed, written.Failed...)
	g.logGamelist(written)
	if len(res.Written) > 0 {
		g.fire(ctx, l, firmware.CondGamelistChanged)
	}
	g.log.Info("gamelists refreshed", "platform", platformID, "games", res.Games, "written", len(res.Written), "failed", len(res.Failed))
	return res, finish(ctx, nil)
}

func (g *Grout) writeLauncher() {
	if g.lists == nil {
		return
	}
	res, err := g.lists.Launcher(g.root)
	if err != nil {
		g.log.Warn("launcher entry not written", "err", err)
		return
	}
	if len(res.Written) > 0 {
		g.fire(g.root, g.snapshot(), firmware.CondGamelistChanged)
	}
}

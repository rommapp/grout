package cli

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"

	"grout/core/config"
	"grout/core/romm"
)

type statusReport struct {
	ConfigSource  string `json:"config_source"`
	Host          string `json:"host"`
	ServerVersion string `json:"server_version"`
	Snapshots     bool   `json:"snapshots"`
	Ready         bool   `json:"ready"`
	Error         string `json:"error"`
}

func status(ctx context.Context, env Env, args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}
	var r statusReport
	code := checkStatus(ctx, env.Dir, env.LogLevel, &r)
	enc := json.NewEncoder(env.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(r)
	return code
}

func checkStatus(ctx context.Context, dir string, levels *slog.LevelVar, r *statusReport) int {
	cfg, src, err := config.Load(dir)
	r.ConfigSource = src.String()
	if err != nil {
		r.Error = err.Error()
		return ExitNotConfigured
	}
	if levels != nil {
		levels.Set(cfg.LogLevel.Slog())
	}
	if len(cfg.Hosts) == 0 {
		r.Error = "no RomM host configured"
		return ExitNotConfigured
	}
	host := cfg.Hosts[0]
	r.Host = host.URL()
	client := romm.New(romm.Options{
		BaseURL:            host.URL(),
		AuthHeader:         host.AuthHeader(),
		InsecureSkipVerify: host.InsecureSkipVerify,
		Timeout:            cfg.ApiTimeout.Duration(),
	})
	hb, err := client.Heartbeat(ctx)
	if err != nil {
		r.Error = err.Error()
		return ExitServer
	}
	r.ServerVersion = hb.System.Version
	r.Snapshots = hb.SaveSync.Snapshots
	if err := hb.RequireSnapshots(); err != nil {
		r.Error = err.Error()
		return ExitTooOld
	}
	r.Ready = true
	return ExitOK
}

package cli

import (
	"context"
	"encoding/json"
	"log/slog"

	"grout/core/config"
	"grout/core/romm"

	"github.com/spf13/cobra"
)

type statusReport struct {
	ConfigSource  string `json:"config_source"`
	Host          string `json:"host"`
	ServerVersion string `json:"server_version"`
	Snapshots     bool   `json:"snapshots"`
	Ready         bool   `json:"ready"`
	Error         string `json:"error"`
}

func newStatusCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report the config source and whether the RomM server supports save channels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var r statusReport
			code := checkStatus(cmd.Context(), env.Dir, env.LogLevel, &r)
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(r); err != nil {
				return err
			}
			if code != ExitOK {
				return &exitError{code: code}
			}
			return nil
		},
	}
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

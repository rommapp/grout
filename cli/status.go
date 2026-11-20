package cli

import (
	"context"
	"encoding/json"
	"errors"
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
	sess, code, err := connect(ctx, dir, levels)
	r.ConfigSource = sess.src.String()
	r.Host = sess.hostURL
	r.ServerVersion = sess.hb.System.Version
	r.Snapshots = sess.hb.SaveSync.Snapshots
	if err != nil {
		r.Error = err.Error()
		return code
	}
	r.Ready = true
	return ExitOK
}

type session struct {
	cfg     *config.Config
	src     config.Source
	hostURL string
	client  *romm.Client
	hb      romm.Heartbeat
}

func connect(ctx context.Context, dir string, levels *slog.LevelVar) (*session, int, error) {
	s := &session{}
	cfg, src, err := config.Load(dir)
	s.src = src
	if err != nil {
		return s, ExitNotConfigured, err
	}
	s.cfg = cfg
	if levels != nil {
		levels.Set(cfg.LogLevel.Slog())
	}
	if len(cfg.Hosts) == 0 {
		return s, ExitNotConfigured, errors.New("no RomM host configured")
	}
	host := cfg.Hosts[0]
	s.hostURL = host.URL()
	s.client = romm.New(romm.Options{
		BaseURL:            host.URL(),
		AuthHeader:         host.AuthHeader(),
		InsecureSkipVerify: host.InsecureSkipVerify,
		Timeout:            cfg.ApiTimeout.Duration(),
	})
	hb, err := s.client.Heartbeat(ctx)
	if err != nil {
		return s, ExitServer, err
	}
	s.hb = hb
	if err := hb.RequireSnapshots(); err != nil {
		return s, ExitTooOld, err
	}
	return s, ExitOK, nil
}

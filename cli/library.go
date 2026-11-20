package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"grout/core/firmware"
	"grout/core/firmware/host"
	"grout/core/library"

	"github.com/spf13/cobra"
)

type libraryReport struct {
	Firmware string                 `json:"firmware"`
	Refresh  *library.RefreshResult `json:"refresh,omitempty"`
	Scan     *library.ScanResult    `json:"scan,omitempty"`
	Error    string                 `json:"error"`
}

func newLibraryCmd(env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Work with the local game library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return &exitError{code: ExitUsage}
		},
	}
	var full bool
	refresh := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the library from RomM, scan local ROM folders and print a JSON summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var r libraryReport
			code := refreshLibrary(cmd.Context(), env, full, &r)
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
	refresh.Flags().BoolVar(&full, "full", false, "ignore stored cursors and fetch everything again")
	cmd.AddCommand(refresh)
	return cmd
}

func refreshLibrary(ctx context.Context, env Env, full bool, r *libraryReport) int {
	getenv := env.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	sess, code, err := connect(ctx, env.Dir, env.LogLevel)
	if err != nil {
		r.Error = err.Error()
		return code
	}
	id, err := firmware.Active(getenv)
	if err != nil {
		r.Error = err.Error()
		return ExitNotConfigured
	}
	r.Firmware = id
	def, err := firmware.Load(id)
	if err != nil {
		r.Error = err.Error()
		return ExitNotConfigured
	}
	fw, err := firmware.Resolve(def, host.OS(getenv), firmware.Overrides{})
	if err != nil {
		r.Error = err.Error()
		return ExitNotConfigured
	}
	st, err := library.Open(env.Dir)
	if err != nil {
		r.Error = err.Error()
		return ExitServer
	}
	defer func() {
		if err := st.Close(); err != nil {
			slog.Warn("closing library store", "error", err)
		}
	}()
	lib := library.New(st, sess.client, sess.cfg, fw)
	res, err := lib.Refresh(ctx, library.RefreshOptions{Full: full}, nil)
	r.Refresh = &res
	if err != nil {
		r.Error = err.Error()
		return ExitServer
	}
	scan, err := lib.Scan(ctx, nil)
	if err != nil {
		r.Error = fmt.Sprintf("scan: %v", err)
		return ExitServer
	}
	r.Scan = &scan
	return ExitOK
}

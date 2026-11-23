package cli

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/rommapp/grout/api"

	"github.com/spf13/cobra"
)

func codeOfEnvelope(raw []byte) api.Code {
	var e struct {
		OK    bool           `json:"ok"`
		Error *api.ErrorBody `json:"error"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return api.CodeInternal
	}
	if e.Error != nil {
		return e.Error.Code
	}
	if !e.OK {
		return api.CodeInternal
	}
	return ""
}

func indented(raw []byte) []byte {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		buf.Reset()
		buf.Write(raw)
	}
	buf.WriteByte('\n')
	return buf.Bytes()
}

func print(env Env, raw []byte) ExitCode {
	env.Stdout.Write(indented(raw))
	return exitFor(codeOfEnvelope(raw))
}

func openOptions(env Env) api.OpenOptions {
	return api.OpenOptions{
		DataDir:  env.Dir,
		Firmware: api.FirmwareSpec{ID: env.Firmware},
		Env:      env.Environ,
		Client:   env.Client,
	}
}

func withServer(ctx context.Context, env Env, exitActions bool, fn func(s *api.Server) ExitCode) ExitCode {
	s, err := api.Open(ctx, openOptions(env))
	if err != nil {
		return print(env, api.ErrorEnvelope(err))
	}
	defer s.Close()
	code := fn(s)
	if exitActions {
		runExitActions(ctx, env, s)
	}
	return code
}

func runExitActions(ctx context.Context, env Env, s *api.Server) {
	raw := s.Call(ctx, "exit_actions.run", nil, nil)
	var e struct {
		OK     bool `json:"ok"`
		Result struct {
			Failed []json.RawMessage `json:"failed"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &e); err != nil || !e.OK || len(e.Result.Failed) > 0 {
		env.Stderr.Write(indented(raw))
	}
}

func invoke(ctx context.Context, env Env, method string, args []byte) ExitCode {
	return invokeWith(ctx, env, false, method, args)
}

func invokeWith(ctx context.Context, env Env, exitActions bool, method string, args []byte) ExitCode {
	return withServer(ctx, env, exitActions, func(s *api.Server) ExitCode {
		return print(env, s.Call(ctx, method, args, nil))
	})
}

func finish(code ExitCode) error {
	if code == ExitOK {
		return nil
	}
	return &exitError{code: code}
}

func newCallCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:    "call <method> [json]",
		Short:  "Call any contract method with a JSON argument and print its envelope",
		Hidden: true,
		Args:   cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var payload []byte
			if len(args) == 2 {
				payload = []byte(args[1])
			}
			return finish(invoke(cmd.Context(), env, args[0], payload))
		},
	}
}

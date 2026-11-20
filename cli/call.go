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

func print(env Env, raw []byte) ExitCode {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		buf.Reset()
		buf.Write(raw)
	}
	buf.WriteByte('\n')
	env.Stdout.Write(buf.Bytes())
	return exitFor(codeOfEnvelope(raw))
}

func invoke(ctx context.Context, env Env, method string, args []byte) ExitCode {
	s, err := api.Open(ctx, api.OpenOptions{
		DataDir:  env.Dir,
		Firmware: api.FirmwareSpec{ID: env.Firmware},
		Env:      env.Environ,
	})
	if err != nil {
		return print(env, api.ErrorEnvelope(err))
	}
	raw := s.Call(ctx, method, args, nil)
	s.Close()
	return print(env, raw)
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

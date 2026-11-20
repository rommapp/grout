package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/rommapp/grout/api"

	"github.com/spf13/cobra"
)

func newAuthCmds(env Env) []*cobra.Command {
	return []*cobra.Command{newPairCmd(env), newLoginCmd(env), newLogoutCmd(env)}
}

func defaultDeviceName(env Env) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return env.Firmware
}

func pairOutcomeExit(raw []byte) ExitCode {
	var e struct {
		Result struct {
			Outcome api.PairOutcome `json:"outcome"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &e) != nil {
		return ExitServer
	}
	switch e.Result.Outcome {
	case api.PairOutcomeApproved:
		return ExitOK
	case api.PairOutcomeDenied, api.PairOutcomeExpired:
		return ExitUnauthorized
	}
	return ExitServer
}

func pairFlow(ctx context.Context, env Env, name string) ExitCode {
	payload, err := json.Marshal(api.PairStartArgs{DeviceName: name})
	if err != nil {
		return print(env, api.ErrorEnvelope(err))
	}
	s, err := api.Open(ctx, openOptions(env))
	if err != nil {
		return print(env, api.ErrorEnvelope(err))
	}
	defer s.Close()
	if code := print(env, s.Call(ctx, "auth.pair_start", payload, nil)); code != ExitOK {
		return code
	}
	raw := s.Call(ctx, "auth.pair_wait", nil, nil)
	if code := print(env, raw); code != ExitOK {
		return code
	}
	return pairOutcomeExit(raw)
}

func newPairCmd(env Env) *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "pair",
		Short: "Pair this device with RomM using a code you approve in the browser",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n := name
			if n == "" {
				n = defaultDeviceName(env)
			}
			return finish(pairFlow(cmd.Context(), env, n))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "device name shown in RomM (default: the hostname)")
	return cmd
}

func newLoginCmd(env Env) *cobra.Command {
	var code string
	cmd := &cobra.Command{
		Use:   "login --code <code>",
		Short: "Sign in with a RomM client API token code",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := json.Marshal(api.ExchangeCodeArgs{Code: code})
			if err != nil {
				return err
			}
			return finish(invoke(cmd.Context(), env, "auth.exchange_code", payload))
		},
	}
	cmd.Flags().StringVar(&code, "code", "", "exchange code from RomM")
	_ = cmd.MarkFlagRequired("code")
	return cmd
}

func newLogoutCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Forget the stored token for this host",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return finish(invoke(cmd.Context(), env, "auth.logout", nil))
		},
	}
}

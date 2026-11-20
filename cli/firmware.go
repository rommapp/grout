package cli

import (
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/rommapp/grout/api"

	"github.com/spf13/cobra"
)

func newFirmwareCmd(env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "firmware",
		Short: "Work with firmware definitions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env.Stderr.Write([]byte(cmd.UsageString()))
			return &exitError{code: ExitUsage}
		},
	}
	var hostMode bool
	validate := &cobra.Command{
		Use:   "validate <file> --host",
		Short: "Validate a host firmware definition file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hostMode {
				return errors.New("only host definitions can be validated; pass --host")
			}
			data, err := os.ReadFile(args[0])
			if err != nil {
				return finish(printFailure(env, api.CodeBadRequest, err.Error()))
			}
			msgs := api.ValidateDefinition(data)
			if len(msgs) == 0 {
				return finish(print(env, []byte(`{"ok":true}`)))
			}
			return finish(printFailure(env, api.CodeBadDefinition, strings.Join(msgs, "\n")))
		},
	}
	validate.Flags().BoolVar(&hostMode, "host", false, "validate as a host definition")
	cmd.AddCommand(validate)
	return cmd
}

func printFailure(env Env, code api.Code, message string) ExitCode {
	raw, err := json.Marshal(api.Envelope{Error: &api.ErrorBody{Code: code, Message: message}})
	if err != nil {
		return ExitServer
	}
	return print(env, raw)
}

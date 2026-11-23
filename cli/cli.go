package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rommapp/grout/api"

	"github.com/spf13/cobra"
)

type Env struct {
	Dir      string
	Stdout   io.Writer
	Stderr   io.Writer
	Firmware string
	Environ  map[string]string
	Client   api.ClientSpec
}

type exitError struct {
	code ExitCode
}

func (e *exitError) Error() string {
	return fmt.Sprintf("exit %d", e.code)
}

func Run(ctx context.Context, env Env, args []string) ExitCode {
	root := newRoot(env)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return ExitOK
	}
	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code
	}
	fmt.Fprintf(env.Stderr, "grout: %v\n\n%s", err, root.UsageString())
	return ExitUsage
}

func newRoot(env Env) *cobra.Command {
	root := &cobra.Command{
		Use:           "grout",
		Short:         "RomM client for retro handhelds",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(env.Stderr, cmd.UsageString())
			return &exitError{code: ExitUsage}
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	root.AddCommand(newStatusCmd(env))
	root.AddCommand(newLibraryCmd(env))
	root.AddCommand(newFirmwareCmd(env))
	root.AddCommand(newFetchCmd(env))
	root.AddCommand(newGamelistCmd(env))
	root.AddCommand(newAuthCmds(env)...)
	root.AddCommand(newCallCmd(env))
	return root
}

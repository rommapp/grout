package cli

import (
	"encoding/json"

	"github.com/rommapp/grout/api"

	"github.com/spf13/cobra"
)

func newGamelistCmd(env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gamelist",
		Short: "Work with the gamelists on the card",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env.Stderr.Write([]byte(cmd.UsageString()))
			return &exitError{code: ExitUsage}
		},
	}
	var exitActions bool
	refresh := &cobra.Command{
		Use:   "refresh [platform-id]",
		Short: "Rewrite the gamelist entries for downloaded games",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var a api.GamelistRefreshArgs
			if len(args) == 1 {
				id, err := positiveArg("platform-id", args[0])
				if err != nil {
					return err
				}
				a.PlatformID = id
			}
			payload, err := json.Marshal(a)
			if err != nil {
				return err
			}
			return finish(invokeWith(cmd.Context(), env, exitActions, "gamelist.refresh", payload))
		},
	}
	refresh.Flags().BoolVar(&exitActions, "exit-actions", false, exitActionsUsage)
	cmd.AddCommand(refresh)
	return cmd
}

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newLibraryCmd(env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Work with the local game library",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env.Stderr.Write([]byte(cmd.UsageString()))
			return &exitError{code: ExitUsage}
		},
	}
	var full bool
	refresh := &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the library from RomM, scan local ROM folders and print the result",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			payload, err := json.Marshal(map[string]bool{"full": full, "scan": true})
			if err != nil {
				return err
			}
			return finish(invoke(cmd.Context(), env, "library.refresh", payload))
		},
	}
	refresh.Flags().BoolVar(&full, "full", false, "ignore stored cursors and fetch everything again")
	cmd.AddCommand(refresh)
	return cmd
}

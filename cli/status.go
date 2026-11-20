package cli

import "github.com/spf13/cobra"

func newStatusCmd(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Report the config source and whether the RomM server supports save channels",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return finish(invoke(cmd.Context(), env, "status", nil))
		},
	}
}

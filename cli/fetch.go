package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/rommapp/grout/api"

	"github.com/spf13/cobra"
)

func positiveArg(name, s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", name, s)
	}
	return n, nil
}

func newFetchCmd(env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Download games, art and BIOS files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env.Stderr.Write([]byte(cmd.UsageString()))
			return &exitError{code: ExitUsage}
		},
	}
	var file int
	game := &cobra.Command{
		Use:   "game <game-id>",
		Short: "Download a game and its art onto the card",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := positiveArg("game-id", args[0])
			if err != nil {
				return err
			}
			if file < 0 {
				return fmt.Errorf("--file must not be negative, got %d", file)
			}
			return fetchCall(cmd, env, "fetch.game", api.FetchGameArgs{GameID: id, FileID: file})
		},
	}
	game.Flags().IntVar(&file, "file", 0, "pick this version (file id) of a game with several")
	art := &cobra.Command{
		Use:   "art <game-id>",
		Short: "Download the art for a downloaded game",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := positiveArg("game-id", args[0])
			if err != nil {
				return err
			}
			return fetchCall(cmd, env, "fetch.art", api.GameIDArgs{GameID: id})
		},
	}
	bios := &cobra.Command{
		Use:   "bios <platform-id>",
		Short: "Download the BIOS files for a platform",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := positiveArg("platform-id", args[0])
			if err != nil {
				return err
			}
			return fetchCall(cmd, env, "fetch.bios", api.PlatformIDArgs{PlatformID: id})
		},
	}
	cmd.AddCommand(game, art, bios)
	return cmd
}

func fetchCall(cmd *cobra.Command, env Env, method string, args any) error {
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return finish(invoke(cmd.Context(), env, method, payload))
}

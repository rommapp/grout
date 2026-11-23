package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

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
	var gameActions, gamesActions, artActions bool
	var jobs int
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
			return fetchCall(cmd, env, gameActions, "fetch.game", api.FetchGameArgs{GameID: id, FileID: file})
		},
	}
	game.Flags().IntVar(&file, "file", 0, "pick this version (file id) of a game with several")
	game.Flags().BoolVar(&gameActions, "exit-actions", false, exitActionsUsage)
	games := &cobra.Command{
		Use:   "games <game-id>...",
		Short: "Download several games and their art onto the card",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ids := make([]int, len(args))
			for i, a := range args {
				id, err := positiveArg("game-id", a)
				if err != nil {
					return err
				}
				ids[i] = id
			}
			if jobs < 1 {
				return fmt.Errorf("--jobs must be at least 1, got %d", jobs)
			}
			return finish(fetchGames(cmd.Context(), env, gamesActions, jobs, ids))
		},
	}
	games.Flags().IntVar(&jobs, "jobs", 2, "download this many games at once")
	games.Flags().BoolVar(&gamesActions, "exit-actions", false, exitActionsUsage)
	art := &cobra.Command{
		Use:   "art <game-id>",
		Short: "Download the art for a downloaded game",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := positiveArg("game-id", args[0])
			if err != nil {
				return err
			}
			return fetchCall(cmd, env, artActions, "fetch.art", api.GameIDArgs{GameID: id})
		},
	}
	art.Flags().BoolVar(&artActions, "exit-actions", false, exitActionsUsage)
	bios := &cobra.Command{
		Use:   "bios <platform-id>",
		Short: "Download the BIOS files for a platform",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := positiveArg("platform-id", args[0])
			if err != nil {
				return err
			}
			return fetchCall(cmd, env, false, "fetch.bios", api.PlatformIDArgs{PlatformID: id})
		},
	}
	cmd.AddCommand(game, games, art, bios)
	return cmd
}

const exitActionsUsage = "run the firmware's exit actions once the command is done"

func fetchCall(cmd *cobra.Command, env Env, exitActions bool, method string, args any) error {
	payload, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return finish(invokeWith(cmd.Context(), env, exitActions, method, payload))
}

func fetchGames(ctx context.Context, env Env, exitActions bool, jobs int, ids []int) ExitCode {
	return withServer(ctx, env, exitActions, func(s *api.Server) ExitCode {
		raws := make([][]byte, len(ids))
		sem := make(chan struct{}, jobs)
		var wg sync.WaitGroup
		for i, id := range ids {
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				payload, err := json.Marshal(api.FetchGameArgs{GameID: id})
				if err != nil {
					raws[i] = api.ErrorEnvelope(err)
					return
				}
				raws[i] = s.Call(ctx, "fetch.game", payload, nil)
			}()
		}
		wg.Wait()
		code := ExitOK
		entries := make([]json.RawMessage, len(ids))
		for i, raw := range raws {
			if c := exitFor(codeOfEnvelope(raw)); code == ExitOK {
				code = c
			}
			entries[i] = withGameID(ids[i], raw)
		}
		out, err := json.Marshal(struct {
			Games []json.RawMessage `json:"games"`
		}{entries})
		if err != nil {
			return print(env, api.ErrorEnvelope(err))
		}
		env.Stdout.Write(indented(out))
		return code
	})
}

func withGameID(id int, raw []byte) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) < 2 || trimmed[0] != '{' {
		return json.RawMessage(fmt.Sprintf(`{"game_id":%d,"ok":false}`, id))
	}
	head := fmt.Sprintf(`{"game_id":%d`, id)
	if string(bytes.TrimSpace(trimmed[1:])) != "}" {
		head += ","
	}
	return json.RawMessage(head + string(trimmed[1:]))
}

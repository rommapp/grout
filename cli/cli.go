package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

type Env struct {
	Dir      string
	Stdout   io.Writer
	Stderr   io.Writer
	LogLevel *slog.LevelVar
}

const usage = `usage: grout <command>

commands:
  status    report the config source and whether the RomM server supports save channels
`

func Run(ctx context.Context, env Env, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, usage)
		return ExitUsage
	}
	switch args[0] {
	case "status":
		return status(ctx, env, args[1:])
	default:
		fmt.Fprintf(env.Stderr, "grout: unknown command %q\n\n%s", args[0], usage)
		return ExitUsage
	}
}

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"grout/cli"

	_ "github.com/BrandonKowalski/certifiable"
)

func main() {
	os.Exit(run())
}

func run() int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "grout:", err)
		return cli.ExitUsage
	}
	level := new(slog.LevelVar)
	level.Set(slog.LevelError)
	if logFile, err := openLog(dir); err != nil {
		fmt.Fprintln(os.Stderr, "grout: logging disabled:", err)
	} else {
		defer logFile.Close()
		slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: level})))
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "grout v3: no front end yet")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, cli.Env{Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr, LogLevel: level}, os.Args[1:])
}

func openLog(dir string) (*os.File, error) {
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(logs, 0755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(logs, "grout.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

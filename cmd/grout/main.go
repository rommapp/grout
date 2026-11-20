package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rommapp/grout/api"
	"github.com/rommapp/grout/cli"
	"github.com/rommapp/grout/core/firmware"
	"github.com/rommapp/grout/version"

	_ "github.com/BrandonKowalski/certifiable"
)

func main() {
	os.Exit(run())
}

func run() int {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "grout:", err)
		return int(cli.ExitUsage)
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "grout v3: no front end yet")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fw, err := firmware.Active(os.Getenv)
	if err != nil {
		fw = ""
	}
	environ := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			environ[k] = v
		}
	}
	return int(cli.Run(ctx, cli.Env{Dir: dir, Stdout: os.Stdout, Stderr: os.Stderr, Firmware: fw, Environ: environ, Client: api.ClientSpec{Name: "grout", Version: version.Version}}, os.Args[1:]))
}

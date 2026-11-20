package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/rommapp/grout/test/mockroot"
)

func main() {
	fw := flag.String("firmware", "", "firmware id")
	device := flag.String("device", "", "device from the firmware's rules (default: the definition's default)")
	platforms := flag.String("platforms", "", "comma separated RomM slugs (default: every row)")
	force := flag.Bool("force", false, "build into a directory that is not empty")
	flag.Parse()
	if *fw == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: mkroot --firmware <id> [--device <device>] [--platforms a,b] [--force] <dir>")
		os.Exit(1)
	}
	o := mockroot.Options{Firmware: *fw, Device: *device, Force: *force}
	if *platforms != "" {
		o.Platforms = strings.Split(*platforms, ",")
	}
	env, err := mockroot.Build(flag.Arg(0), o)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%s=%s\n", k, env[k])
	}
}

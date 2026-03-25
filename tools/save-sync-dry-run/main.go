package main

import (
	"fmt"
	"grout/cache"
	"grout/internal"
	"grout/romm"
	"grout/sync"
	"os"
	"strings"
	"time"
)

func main() {

	config, err := internal.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if len(config.Hosts) == 0 {
		fmt.Fprintln(os.Stderr, "No hosts configured in config.json")
		os.Exit(1)
	}

	host := config.Hosts[0]
	if host.DeviceID == "" {
		fmt.Fprintln(os.Stderr, "No device_id set on host. Register a device first.")
		os.Exit(1)
	}

	if err := cache.InitCacheManager(host, config); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init cache: %v\n", err)
		os.Exit(1)
	}
	defer cache.GetCacheManager().Close()

	client := romm.NewClientFromHost(host, config.ApiTimeout)

	fmt.Printf("Host:     %s\n", host.URL())
	fmt.Printf("Device:   %s\n", host.DeviceID)
	fmt.Println()

	result, err := sync.ResolveSaveSync(client, config, host.DeviceID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to resolve sync: %v\n", err)
		os.Exit(1)
	}

	items := result.Items
	fmt.Printf("Session ID: %d\n", result.SessionID)
	fmt.Printf("Total sync items: %d\n\n", len(items))

	if len(items) == 0 {
		fmt.Println("Nothing to sync.")
		return
	}

	printTable(items, host.DeviceID)
}

type row struct {
	action        string
	rom           string
	localFile     string
	localMtime    string
	remoteID      string
	remoteUpdated string
	isCurrent     string
	slot          string
}

func printTable(items []sync.SyncItem, deviceID string) {
	headers := row{"ACTION", "ROM", "LOCAL FILE", "LOCAL MTIME", "REMOTE ID", "REMOTE UPDATED", "CURRENT", "SLOT"}

	var rows []row
	for _, item := range items {
		r := row{
			action:        strings.ToUpper(item.Action.String()),
			rom:           item.LocalSave.RomName,
			localFile:     item.LocalSave.FileName,
			localMtime:    "-",
			remoteID:      "-",
			remoteUpdated: "-",
			isCurrent:     "-",
			slot:          "-",
		}

		if item.LocalSave.FilePath != "" {
			if info, err := os.Stat(item.LocalSave.FilePath); err == nil {
				r.localMtime = info.ModTime().Local().Format(time.DateTime)
			}
		}

		if item.RemoteSave != nil {
			r.remoteID = fmt.Sprintf("%d", item.RemoteSave.ID)
			r.remoteUpdated = item.RemoteSave.UpdatedAt.Local().Format(time.DateTime)
			if item.RemoteSave.Slot != nil {
				r.slot = *item.RemoteSave.Slot
			}
			for _, ds := range item.RemoteSave.DeviceSyncs {
				if ds.DeviceID == deviceID {
					if ds.IsCurrent {
						r.isCurrent = "yes"
					} else {
						r.isCurrent = "no"
					}
					break
				}
			}
		}

		if r.localFile == "" && item.RemoteSave != nil {
			r.localFile = "(" + item.RemoteSave.FileName + ")"
		}

		rows = append(rows, r)
	}

	widths := [8]int{
		len(headers.action), len(headers.rom), len(headers.localFile),
		len(headers.localMtime), len(headers.remoteID), len(headers.remoteUpdated),
		len(headers.isCurrent), len(headers.slot),
	}
	for _, r := range rows {
		fields := [8]string{r.action, r.rom, r.localFile, r.localMtime, r.remoteID, r.remoteUpdated, r.isCurrent, r.slot}
		for i, f := range fields {
			if len(f) > widths[i] {
				widths[i] = len(f)
			}
		}
	}

	fmtStr := fmt.Sprintf(" %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds │ %%-%ds ",
		widths[0], widths[1], widths[2], widths[3], widths[4], widths[5], widths[6], widths[7])

	sep := "─" + strings.Repeat("─", widths[0]) + "─┼─" +
		strings.Repeat("─", widths[1]) + "─┼─" +
		strings.Repeat("─", widths[2]) + "─┼─" +
		strings.Repeat("─", widths[3]) + "─┼─" +
		strings.Repeat("─", widths[4]) + "─┼─" +
		strings.Repeat("─", widths[5]) + "─┼─" +
		strings.Repeat("─", widths[6]) + "─┼─" +
		strings.Repeat("─", widths[7]) + "─"

	h := fmt.Sprintf(fmtStr, headers.action, headers.rom, headers.localFile,
		headers.localMtime, headers.remoteID, headers.remoteUpdated, headers.isCurrent, headers.slot)
	fmt.Println(h)
	fmt.Println(sep)

	for _, r := range rows {
		fmt.Printf(fmtStr+"\n", r.action, r.rom, r.localFile, r.localMtime, r.remoteID, r.remoteUpdated, r.isCurrent, r.slot)
	}

	counts := map[string]int{}
	for _, item := range items {
		counts[strings.ToUpper(item.Action.String())]++
	}
	fmt.Println()
	for action, count := range counts {
		fmt.Printf("  %s: %d\n", action, count)
	}
}

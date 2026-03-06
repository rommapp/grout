// create-conflict forces a save sync conflict for a given ROM ID. It downloads
// the existing save from the server and re-uploads it via PUT /api/saves/{id},
// which updates the save's updatedAt in place without creating a new record.
// This makes remoteChanged=true. On the device, the local save file must also
// be touched so its mtime is newer than lastSyncedAt, making localChanged=true
// and triggering a conflict.
//
// Works with both slotted and non-slotted saves.
//
// Usage:
//
//	go run tools/create-conflict/main.go <rom_id> [slot]
package main

import (
	"fmt"
	"grout/internal"
	"grout/romm"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run tools/create-conflict/main.go <rom_id> [slot]")
		os.Exit(1)
	}

	romID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid rom_id: %s\n", os.Args[1])
		os.Exit(1)
	}

	var slotFilter string
	if len(os.Args) >= 3 {
		slotFilter = os.Args[2]
	}

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

	client := romm.NewClientFromHost(host, config.ApiTimeout)

	fmt.Printf("Host:     %s\n", host.URL())
	fmt.Printf("Device:   %s\n", host.DeviceID)
	fmt.Printf("ROM ID:   %d\n", romID)
	if slotFilter != "" {
		fmt.Printf("Slot:     %s\n", slotFilter)
	}
	fmt.Println()

	// Find existing saves for this ROM with this device's sync history
	query := romm.SaveQuery{RomID: romID, DeviceID: host.DeviceID}
	if slotFilter != "" {
		query.Slot = slotFilter
	}
	saves, err := client.GetSaves(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch saves: %v\n", err)
		os.Exit(1)
	}

	// Find the save this device has synced
	var targetSave *romm.Save
	var deviceSync *romm.DeviceSaveSync
	for i := range saves {
		for j := range saves[i].DeviceSyncs {
			ds := &saves[i].DeviceSyncs[j]
			if ds.DeviceID == host.DeviceID && !ds.IsUntracked {
				targetSave = &saves[i]
				deviceSync = ds
				break
			}
		}
		if targetSave != nil {
			break
		}
	}

	if targetSave == nil {
		fmt.Fprintln(os.Stderr, "This device has no sync history for ROM", romID)
		if slotFilter != "" {
			fmt.Fprintf(os.Stderr, "  (filtered to slot %q)\n", slotFilter)
		}
		fmt.Fprintln(os.Stderr, "Run a save sync on the device first to establish a baseline, then run this tool again.")
		os.Exit(1)
	}

	slot := "default"
	if targetSave.Slot != nil {
		slot = *targetSave.Slot
	}

	fmt.Printf("Found save: ID=%d, file=%s, slot=%s, emulator=%s\n", targetSave.ID, targetSave.FileName, slot, targetSave.Emulator)
	fmt.Printf("  updatedAt:    %s\n", targetSave.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("  lastSyncedAt: %s\n", deviceSync.LastSyncedAt.Format(time.RFC3339))
	fmt.Printf("  isCurrent:    %v\n\n", deviceSync.IsCurrent)

	// Download the existing save content
	fmt.Println("Step 1: Downloading existing save content...")
	saveData, err := client.DownloadSave(targetSave.DownloadPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download save: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Downloaded %d bytes\n", len(saveData))

	// Write to temp file
	tmpDir, err := os.MkdirTemp("", "create-conflict-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	savePath := filepath.Join(tmpDir, targetSave.FileName)
	if err := os.WriteFile(savePath, saveData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write temp save: %v\n", err)
		os.Exit(1)
	}

	// PUT the save back to the same ID — updates updatedAt in place
	fmt.Println("\nStep 2: Re-uploading save via PUT (advances updatedAt in place)...")
	updated, err := client.UpdateSave(targetSave.ID, savePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update save: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Old updatedAt: %s\n", targetSave.UpdatedAt.Format(time.RFC3339))
	fmt.Printf("  New updatedAt: %s\n", updated.UpdatedAt.Format(time.RFC3339))

	// Verify the state
	fmt.Println("\nStep 3: Verifying conflict state...")
	verifySaves, err := client.GetSaves(romm.SaveQuery{RomID: romID, DeviceID: host.DeviceID})
	if err == nil {
		for _, s := range verifySaves {
			for _, ds := range s.DeviceSyncs {
				if ds.DeviceID == host.DeviceID {
					fmt.Printf("  Save ID=%d: updatedAt=%s, lastSyncedAt=%s, isCurrent=%v\n",
						s.ID, s.UpdatedAt.Format(time.RFC3339), ds.LastSyncedAt.Format(time.RFC3339), ds.IsCurrent)
					if s.UpdatedAt.After(ds.LastSyncedAt) {
						fmt.Println("  remoteChanged=true (updatedAt > lastSyncedAt)")
					} else {
						fmt.Println("  remoteChanged=false (updatedAt <= lastSyncedAt) — conflict won't trigger!")
					}
				}
			}
		}
	}

	fmt.Printf("\nServer-side conflict set up for ROM %d.\n", romID)
	fmt.Println("On the device, touch the local save file to make localChanged=true:")
	fmt.Printf("  touch <save_dir>/%s\n", targetSave.FileName)
	fmt.Println("Then run save sync to trigger the conflict resolution screen.")
}

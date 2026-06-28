//go:build dryrun

package sync

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"grout/cfw"
	"grout/internal"
)

// RunScenario executes an offline, self-contained demonstration of a save-sync fix using
// the real resolution functions, so the fixed behavior can be verified without a live RomM
// server or a handheld. It is compiled only under the "dryrun" build tag, so none of this
// links into the device binary. Invoked by the save-sync-dry-run tool's -scenario flag.
func RunScenario(name string, w io.Writer) error {
	switch name {
	case "slot-switch":
		return scenarioSlotSwitch(w)
	case "nextui-keep":
		return scenarioNextUINaming(w, true)
	case "nextui-retroarch":
		return scenarioNextUINaming(w, false)
	case "all":
		for _, s := range []string{"slot-switch", "nextui-keep", "nextui-retroarch"} {
			if err := RunScenario(s, w); err != nil {
				return err
			}
			fmt.Fprintln(w)
		}
		return nil
	default:
		return fmt.Errorf("unknown scenario %q (want: slot-switch, nextui-keep, nextui-retroarch, all)", name)
	}
}

func passFail(ok bool) string {
	if ok {
		return "PASS"
	}
	return "FAIL"
}

// scenarioSlotSwitch reproduces issue #250: a ROM whose last-synced slot was recorded as
// "default" (e.g. from an older grout, or sync with another client) could never be switched
// back to "autosave", because picking autosave used to be discarded as "the default". It
// drives the real buildClientSaveStates / resolveReportedSlot precedence and the real
// Config slot-preference API.
func scenarioSlotSwitch(w io.Writer) error {
	dir, err := os.MkdirTemp("", "grout-dryrun-slot-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	const romID = 6
	const fileName = "Pokemon - Emerald Version (USA, Europe).srm"
	savePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(savePath, []byte("save-data"), 0644); err != nil {
		return err
	}

	local := []LocalSave{{
		RomID:       romID,
		RomName:     "Pokemon - Emerald Version",
		FileName:    fileName,
		FilePath:    savePath,
		EmulatorDir: "mGBA",
	}}
	// The device's recorded last-synced slot for this ROM (from the issue's SQLite dump).
	recorded := map[saveKey]string{{romID: romID, fileName: fileName}: "default"}

	fmt.Fprintln(w, "Scenario #250 — switch a ROM's active save slot to autosave")
	fmt.Fprintf(w, "  ROM %d (%s), recorded slot on this device: %q\n\n", romID, fileName, "default")

	// No explicit preference yet: the sticky recorded slot is reported.
	before := buildClientSaveStates(local, &internal.Config{}, recorded)
	fmt.Fprintf(w, "  no explicit preference        -> reports slot %q  (recorded slot)\n", before[0].Slot)

	// User picks "autosave" in the UI. Pre-fix this was discarded (SlotPreferenceExplicit
	// stayed false) and the recorded "default" kept winning; now it persists and overrides.
	cfg := &internal.Config{}
	cfg.SetSlotPreference(romID, "autosave")
	_, explicit := cfg.SlotPreferenceExplicit(romID)
	after := buildClientSaveStates(local, cfg, recorded)
	fmt.Fprintf(w, "  user picks autosave in the UI -> SlotPreferenceExplicit=%v, reports slot %q  (explicit choice wins)\n", explicit, after[0].Slot)

	ok := explicit && after[0].Slot == "autosave"
	fmt.Fprintf(w, "\n  %s: explicit autosave overrides the recorded slot.\n", passFail(ok))
	if !ok {
		return fmt.Errorf("scenario slot-switch failed: explicit=%v slot=%q", explicit, after[0].Slot)
	}
	return nil
}

// scenarioNextUINaming reproduces issue #245 for one of NextUI's two save-naming styles:
// minarch (keepStyle=true) keeps the full ROM filename incl. extension, the RetroArch-style
// option (keepStyle=false) strips it. It exercises the real read-side candidate matching
// (saveLookupKeys) and the real write-side detection + naming (detectSaveNameStyle /
// downloadSaveFileName).
func scenarioNextUINaming(w io.Writer, keepStyle bool) error {
	// detectSaveNameStyle's fallback consults cfw.GetCFW(); pin it to NextUI so the real
	// detection (and its CFW default) runs exactly as it would on the device.
	os.Setenv("CFW", "NEXTUI")

	dir, err := os.MkdirTemp("", "grout-dryrun-naming-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	const romFile = "Donkey Kong Country (USA) (Rev 2).sfc" // ROM as it sits on disk
	var sibling, scannedSave string
	if keepStyle {
		fmt.Fprintln(w, "Scenario #245 — NextUI minarch save naming (keeps the ROM extension)")
		sibling = "Super Mario World (USA).sfc.sav"
		scannedSave = "Donkey Kong Country (USA) (Rev 2).sfc.sav"
	} else {
		fmt.Fprintln(w, "Scenario #245 — NextUI RetroArch-style save naming (strips the ROM extension)")
		sibling = "Super Mario World (USA).srm"
		scannedSave = "Donkey Kong Country (USA) (Rev 2).srm"
	}
	// Seed an existing sibling save so write-side detection has the device convention to learn.
	if err := os.WriteFile(filepath.Join(dir, sibling), []byte("x"), 0644); err != nil {
		return err
	}

	// expected_basename in grout's cache is the ROM basename without its extension.
	romBase := cfw.SaveBasename(false, romFile)
	saveExt := strings.TrimPrefix(filepath.Ext(scannedSave), ".")

	fmt.Fprintf(w, "  ROM on disk:           %s\n", romFile)
	fmt.Fprintf(w, "  ROM expected_basename: %s\n", romBase)
	fmt.Fprintf(w, "  existing save in dir:  %s  (sibling for style detection)\n\n", sibling)

	// READ side: a scanned save must resolve back to its ROM via candidate match keys.
	nameNoExt := strings.TrimSuffix(scannedSave, filepath.Ext(scannedSave))
	keys := saveLookupKeys(nameNoExt)
	readOK := false
	for _, k := range keys {
		if k == romBase {
			readOK = true
		}
	}
	fmt.Fprintf(w, "  READ  scanned save: %s\n", scannedSave)
	fmt.Fprintf(w, "        candidates:   %v\n", keys)
	fmt.Fprintf(w, "        resolves to ROM -> %s\n\n", passFail(readOK))

	// WRITE side: a fresh download must be named for the convention the emulator uses.
	keep := detectSaveNameStyle(dir)
	dl := downloadSaveFileName(romFile, "server [2026-01-01_00-00-00]."+saveExt, saveExt, keep)
	writeOK := dl == scannedSave
	fmt.Fprintf(w, "  WRITE detected style: keepRomExt=%v\n", keep)
	fmt.Fprintf(w, "        download writes: %s\n", dl)
	fmt.Fprintf(w, "        matches emulator expectation (%s) -> %s\n", scannedSave, passFail(writeOK))

	ok := readOK && writeOK
	fmt.Fprintf(w, "\n  %s: save round-trips for this naming style.\n", passFail(ok))
	if !ok {
		return fmt.Errorf("scenario nextui naming (keepStyle=%v) failed: read=%v write=%v", keepStyle, readOK, writeOK)
	}
	return nil
}

package bios

import (
	"testing"

	"grout/romm"
)

var psxBIOS = []File{
	{FileName: "scph5500.bin", RelativePath: "scph5500.bin"},
	{FileName: "psxonpsp660.bin", RelativePath: "psxonpsp660.bin", Optional: true},
	{FileName: "bios7.bin", RelativePath: "nds/bios7.bin"},
}

func only(t *testing.T, requirements []Requirement) Requirement {
	t.Helper()
	if len(requirements) != 1 {
		t.Fatalf("got %d requirements, want 1", len(requirements))
	}
	return requirements[0]
}

func TestMatch_ByFileName(t *testing.T) {
	got := only(t, matchAgainst(psxBIOS, []romm.Firmware{{FileName: "scph5500.bin"}}))

	if !got.Known {
		t.Fatal("scph5500.bin is in the tables and must be recognised")
	}
	if got.File.RelativePath != "scph5500.bin" {
		t.Errorf("RelativePath = %q, want the table's", got.File.RelativePath)
	}
}

// RomM stores whatever the uploader typed, so the case of a BIOS name is not
// something to rely on.
func TestMatch_IgnoresCase(t *testing.T) {
	got := only(t, matchAgainst(psxBIOS, []romm.Firmware{{FileName: "SCPH5500.BIN"}}))

	if !got.Known {
		t.Error("a file uploaded in a different case is still the same file")
	}
}

// A file the tables place in a subdirectory must be recognised from its bare
// name, which is how the server usually holds it, and installed to the
// subdirectory the emulator looks in.
func TestMatch_ByBaseNameKeepsTheTablesPath(t *testing.T) {
	got := only(t, matchAgainst(psxBIOS, []romm.Firmware{{FileName: "bios7.bin"}}))

	if !got.Known {
		t.Fatal("bios7.bin must be recognised from its bare name")
	}
	if got.File.RelativePath != "nds/bios7.bin" {
		t.Errorf("RelativePath = %q, want the subdirectory the tables give it", got.File.RelativePath)
	}
}

// The server can hold a file under a path rather than a bare name.
func TestMatch_ByServerPath(t *testing.T) {
	got := only(t, matchAgainst(psxBIOS, []romm.Firmware{
		{FileName: "something-else.bin", FilePath: "nds/bios7.bin"},
	}))

	if !got.Known || got.File.RelativePath != "nds/bios7.bin" {
		t.Errorf("known=%v path=%q, want a match on the server's path", got.Known, got.File.RelativePath)
	}
}

// A file grout has never heard of is still worth downloading: the emulator is
// usually looking for exactly the name the server holds.
func TestMatch_UnknownFileIsStillOffered(t *testing.T) {
	got := only(t, matchAgainst(psxBIOS, []romm.Firmware{{FileName: "mystery.bin"}}))

	if got.Known {
		t.Error("nothing in the tables matched, so Known must be false")
	}
	if got.File.RelativePath != "mystery.bin" {
		t.Errorf("RelativePath = %q, want the server's own name", got.File.RelativePath)
	}
	// Claiming an unrecognised file is optional would tell the user they can
	// skip something the emulator needs.
	if got.File.Optional {
		t.Error("an unmatched file must not be reported as optional")
	}
}

func TestMatch_CarriesOptional(t *testing.T) {
	got := only(t, matchAgainst(psxBIOS, []romm.Firmware{{FileName: "psxonpsp660.bin"}}))

	if !got.Known || !got.File.Optional {
		t.Errorf("known=%v optional=%v, want a recognised optional file", got.Known, got.File.Optional)
	}
}

func TestMatch_KeepsServerOrderAndCount(t *testing.T) {
	firmware := []romm.Firmware{
		{FileName: "mystery.bin"},
		{FileName: "scph5500.bin"},
		{FileName: "psxonpsp660.bin"},
	}

	got := matchAgainst(psxBIOS, firmware)

	if len(got) != len(firmware) {
		t.Fatalf("got %d requirements for %d firmware entries", len(got), len(firmware))
	}
	for i, entry := range firmware {
		if got[i].Firmware.FileName != entry.FileName {
			t.Errorf("position %d is %q, want %q", i, got[i].Firmware.FileName, entry.FileName)
		}
	}
}

// The embedded tables have to actually reach Match, not just matchAgainst.
func TestMatch_UsesTheEmbeddedTables(t *testing.T) {
	got := only(t, Match("psx", []romm.Firmware{{FileName: "scph5500.bin"}}))

	if !got.Known {
		t.Error("scph5500.bin is a known PlayStation BIOS in the embedded tables")
	}
}

func TestMatch_UnknownPlatform(t *testing.T) {
	got := only(t, Match("not-a-console", []romm.Firmware{{FileName: "mystery.bin"}}))

	if got.Known {
		t.Error("a platform with no table entries can match nothing")
	}
	if got.File.RelativePath != "mystery.bin" {
		t.Errorf("RelativePath = %q, want it still downloadable under the server's name", got.File.RelativePath)
	}
}

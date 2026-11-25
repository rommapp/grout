package bios

import (
	"strings"
	"testing"

	"grout/romm"
)

// Every core a platform names must exist in the requirements table, or the
// platform silently loses the subdirectory its BIOS files belong in.
func TestPlatformCores_ReferenceKnownCores(t *testing.T) {
	for platform, cores := range PlatformToLibretroCores {
		for _, core := range cores {
			if _, ok := LibretroCoreToBIOS[strings.TrimSuffix(core, "_libretro")]; !ok {
				t.Errorf("%s names core %q, which core_requirements.json does not know", platform, core)
			}
		}
	}
}

// Cores that read their BIOS from a subdirectory of the BIOS folder. Without a
// platform mapping the files are not recognised and end up at the root of the
// BIOS folder, where the core never looks (rommapp/grout#326).
func TestMatch_SubdirectoryBIOSLandsInCoreFolder(t *testing.T) {
	cases := []struct {
		platform string
		file     string
		want     string
	}{
		{"pc-9800-series", "font.bmp", "np2kai/font.bmp"},
		{"pc-9800-series", "2608_BD.WAV", "np2kai/2608_BD.WAV"},
		{"pc-8000", "n88.rom", "quasi88/n88.rom"},
		{"neo-geo-cd", "neocd_f.rom", "neocd/neocd_f.rom"},
		{"sharp-x68000", "iplrom.dat", "keropi/iplrom.dat"},
		{"x1", "IPLROM.X1", "xmil/IPLROM.X1"},
		{"philips-cd-i", "cdimono1.zip", "same_cdi/bios/cdimono1.zip"},
		{"galaksija", "CHRGEN.BIN", "galaksija/CHRGEN.BIN"},
		{"scummvm", "scummmodern.zip", "scummvm/theme/scummmodern.zip"},
		{"scummvm", "encoding.dat", "scummvm/extra/encoding.dat"},
	}
	for _, tc := range cases {
		t.Run(tc.platform+"/"+tc.file, func(t *testing.T) {
			got := only(t, Match(tc.platform, []romm.Firmware{{FileName: tc.file}}))
			if !got.Known {
				t.Fatalf("%s must be recognised for %s", tc.file, tc.platform)
			}
			if got.File.RelativePath != tc.want {
				t.Errorf("RelativePath = %q, want %q", got.File.RelativePath, tc.want)
			}
		})
	}
}

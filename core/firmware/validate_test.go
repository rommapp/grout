package firmware

import (
	"strings"
	"testing"
	"testing/fstest"
)

var knownForTest = map[string]bool{"gba": true, "psx": true, "snes": true}

const validTOML = `
id = "fw"
name = "Firmware"
maintained = "upstream"
upstream = { repo = "github.com/x/fw", ref = "v1.0" }

[version]
file = { value = "{card}/version.txt", src = "sys/version.txt" }

[paths]
card  = { value = "/mnt/SDCARD", src = "sys/a" }
roms  = { value = "{card}/Roms", src = "sys/b" }
bios  = { value = "{card}/Bios", src = "sys/c" }
saves = { value = "{card}/Saves", src = "sys/d" }

[rom_folders]
style = { value = "tagged", src = "sys/e" }

[saves]
layout = { value = "per-emulator", src = "sys/f" }
naming = { value = "retroarch", src = "sys/g" }

[bios]
default = { value = ["{bios}/{save_folder}/{name}", "{bios}/{rel}"], src = "sys/h" }

[art]
naming = "plain"
cover = { value = "{rom_dir}/.media", src = "sys/i" }

[gamelist]
format = "none"

[platform_sources]
roms = "sys/roms"
saves = "sys/saves"

[platforms]
gba = { roms = ["Game Boy Advance (GBA)"], saves = ["GBA"] }
psx = { roms = [], saves = [] }

[devices]
default = "generic"
rules = [
  { device = "trimui", env = "DEV", equals = "tg5040" },
  { device = "flip", file = "/etc/model", contains = "flip" },
]

[devices.quirks]
flip = { rotation = 90 }

[packaging]
asset = "Grout.zip"
launch_script = "launch.sh"
install_depth = 1
files = ["launch.sh"]
`

func parseValid(t *testing.T, src string) *Definition {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func valid(t *testing.T) *Definition {
	return parseValid(t, validTOML)
}

func check(d *Definition) []error {
	return Validate(d, knownForTest, Registry{}, fstest.MapFS{})
}

func wantErr(t *testing.T, errs []error, parts ...string) {
	t.Helper()
	for _, e := range errs {
		ok := true
		for _, p := range parts {
			if !strings.Contains(e.Error(), p) {
				ok = false
			}
		}
		if ok {
			return
		}
	}
	t.Fatalf("no error containing %q in %v", parts, errs)
}

func TestValidMinimalDefinition(t *testing.T) {
	if errs := Validate(valid(t), knownForTest, Registry{}, fstest.MapFS{}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidCommunityDefinition(t *testing.T) {
	d := parseValid(t, `
id = "c"
name = "C"
maintained = "community"
[paths]
card = { value = "/mnt/SDCARD" }
roms = { value = "{card}/Roms" }
bios = { value = "{card}/Bios" }
saves = { value = "{card}/Saves" }
[packaging]
asset = "a.zip"
launch_script = "l.sh"
install_depth = 1
`)
	if errs := check(d); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateMissingSrc(t *testing.T) {
	d := valid(t)
	d.Paths.Roms.Src = ""
	wantErr(t, check(d), "paths.roms", "src")
}

func TestValidateMissingSrcOnAlternative(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Alts: []Fact{
		{Value: "{card}/BIOS", Src: "a", Until: "2"},
		{Value: "{card}/Bios", Since: "2"},
	}}
	wantErr(t, check(d), "paths.bios[1]", "src")
}

func TestValidateCommunityWithSrc(t *testing.T) {
	d := valid(t)
	d.Maintained = "community"
	d.Upstream.Repo = ""
	d.Upstream.Ref = ""
	d.PlatformSources = nil
	wantErr(t, check(d), "paths.card", "src")
}

func TestValidateCommunityWithUpstream(t *testing.T) {
	d := valid(t)
	d.Maintained = "community"
	for _, f := range []*Fact{&d.Version.File, &d.Paths.Card, &d.Paths.Roms, &d.Paths.Bios, &d.Paths.Saves, &d.RomFolders.Style, &d.Saves.Layout, &d.Saves.Naming, &d.Bios.Default, d.Art.Cover} {
		f.Src = ""
	}
	errs := check(d)
	wantErr(t, errs, "upstream")
	wantErr(t, errs, "platform_sources")
}

func TestValidateUpstreamNeedsRepoAndRef(t *testing.T) {
	d := valid(t)
	d.Upstream.Repo = ""
	d.Upstream.Ref = ""
	errs := check(d)
	wantErr(t, errs, "upstream.repo")
	wantErr(t, errs, "upstream.ref")
}

func TestValidateRequiredIdentity(t *testing.T) {
	d := valid(t)
	d.ID = ""
	d.Name = ""
	d.Maintained = ""
	errs := check(d)
	wantErr(t, errs, "id")
	wantErr(t, errs, "name")
	wantErr(t, errs, "maintained")
}

func TestValidateBadMaintained(t *testing.T) {
	d := valid(t)
	d.Maintained = "vendor"
	wantErr(t, check(d), "maintained", "vendor")
}

func TestValidateUnknownHook(t *testing.T) {
	d := valid(t)
	d.Paths.Roms = Fact{Hook: "fw_roms"}
	wantErr(t, check(d), "paths.roms", "fw_roms")
}

func TestValidateKnownHookAccepted(t *testing.T) {
	d := valid(t)
	d.Paths.Roms = Fact{Hook: "fw_roms"}
	d.Version.Hook = "fw_version"
	d.Version.File = Fact{}
	d.Saves.NamingHook = "fw_naming"
	d.Saves.ExtraFoldersHook = "fw_folders"
	d.Saves.LabelHook = "fw_label"
	d.Devices.Inputs = map[string]Input{"flip": {Hook: "fw_input"}}
	reg := Registry{
		Path:    map[string]PathHook{"fw_roms": nil},
		Version: map[string]VersionHook{"fw_version": nil},
		Naming:  map[string]NamingHook{"fw_naming": nil},
		Folders: map[string]FoldersHook{"fw_folders": nil},
		Label:   map[string]LabelHook{"fw_label": nil},
		Input:   map[string]InputHook{"fw_input": nil},
	}
	if errs := Validate(d, knownForTest, reg, fstest.MapFS{}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateHookInWrongRegistry(t *testing.T) {
	d := valid(t)
	d.Saves.NamingHook = "fw_x"
	d.Saves.ExtraFoldersHook = "fw_x"
	d.Saves.LabelHook = "fw_x"
	d.Version.Hook = "fw_x"
	d.Version.File = Fact{}
	d.Devices.Inputs = map[string]Input{"flip": {Hook: "fw_x"}}
	reg := Registry{Path: map[string]PathHook{"fw_x": nil}}
	errs := Validate(d, knownForTest, reg, fstest.MapFS{})
	wantErr(t, errs, "saves.naming_hook", "fw_x")
	wantErr(t, errs, "saves.extra_folders_hook", "fw_x")
	wantErr(t, errs, "saves.label_hook", "fw_x")
	wantErr(t, errs, "version.hook", "fw_x")
	wantErr(t, errs, "devices.inputs.flip", "fw_x")
}

func TestValidateHookOnValueOnlyField(t *testing.T) {
	d := valid(t)
	d.RomFolders.Style = Fact{Hook: "fw_style"}
	d.Art.Cover = &Fact{Hook: "fw_cover"}
	d.Bios.Default = Fact{Hook: "fw_bios"}
	errs := check(d)
	wantErr(t, errs, "rom_folders.style", "hook")
	wantErr(t, errs, "art.cover", "hook")
	wantErr(t, errs, "bios.default", "hook")
}

func TestValidateValueAndHook(t *testing.T) {
	d := valid(t)
	d.Paths.Roms.Hook = "fw_roms"
	wantErr(t, check(d), "paths.roms", "exactly one")
}

func TestValidateNeitherValueNorHook(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Src: "x"}
	wantErr(t, check(d), "paths.bios", "exactly one")
}

func TestValidateAlternativeValueAndHook(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Alts: []Fact{
		{Value: "{card}/a", Hook: "h", Src: "a", Until: "2"},
		{Value: "{card}/b", Src: "b", Since: "2"},
	}}
	wantErr(t, check(d), "paths.bios[0]", "exactly one")
}

func TestValidateUnknownSlug(t *testing.T) {
	d := valid(t)
	d.Platforms["gbaa"] = Row{Roms: []string{"X"}, Saves: []string{"X"}}
	wantErr(t, check(d), "platforms.gbaa")
}

func TestValidateUnknownTemplateVar(t *testing.T) {
	d := valid(t)
	d.Paths.Roms = Fact{Value: "{card}/{nope}", Src: "x"}
	wantErr(t, check(d), "paths.roms", "nope")
}

func TestValidateTemplateVarOutOfPosition(t *testing.T) {
	d := valid(t)
	d.Paths.Roms = Fact{Value: "{bios}/Roms", Src: "x"}
	wantErr(t, check(d), "paths.roms", "bios")

	d = valid(t)
	d.Paths.Card = Fact{Value: "{card}", Src: "x"}
	wantErr(t, check(d), "paths.card", "card")

	d = valid(t)
	d.Art.Cover = &Fact{Value: "{bios}/x", Src: "x"}
	wantErr(t, check(d), "art.cover", "bios")

	d = valid(t)
	d.Bios.Default = Fact{Value: []string{"{bios}/{rom_dir}"}, Src: "x"}
	wantErr(t, check(d), "bios.default", "rom_dir")

	d = valid(t)
	d.Platforms["gba"] = Row{Roms: []string{"GBA"}, Saves: []string{"GBA"}, Bios: []string{"{bios}/{card}"}}
	wantErr(t, check(d), "platforms.gba", "card")

	d = valid(t)
	d.Gamelist.LauncherEntry = &struct {
		Gamelist string `toml:"gamelist"`
		Path     string `toml:"path"`
	}{Gamelist: "{card}/{name}/gamelist.xml", Path: "./x"}
	wantErr(t, check(d), "gamelist.launcher_entry.gamelist", "name")
}

func TestValidateTemplateVarsAllowedInPosition(t *testing.T) {
	d := valid(t)
	d.Art.Cover = &Fact{Value: "{card}/{roms}/{rom_dir}/{rom_folder}/{system}", Src: "x"}
	d.Platforms["gba"] = Row{Roms: []string{"GBA"}, Saves: []string{"GBA"}, Bios: []string{"{bios}/{name}/{rel}/{save_folder}"}}
	d.Gamelist.LauncherEntry = &struct {
		Gamelist string `toml:"gamelist"`
		Path     string `toml:"path"`
	}{Gamelist: "{card}/{roms}/gamelist.xml", Path: "./x"}
	if errs := check(d); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateBadEnum(t *testing.T) {
	d := valid(t)
	d.RomFolders.Style = Fact{Value: "fancy", Src: "x"}
	wantErr(t, check(d), "rom_folders.style", "fancy")

	d = valid(t)
	d.Saves.Layout = Fact{Value: "flat", Src: "x"}
	wantErr(t, check(d), "saves.layout", "flat")

	d = valid(t)
	d.Saves.Naming = Fact{Value: "odd", Src: "x"}
	wantErr(t, check(d), "saves.naming", "odd")

	d = valid(t)
	d.Art.Naming = "suffix"
	wantErr(t, check(d), "art.naming", "suffix")

	d = valid(t)
	d.Gamelist.Format = "xml"
	wantErr(t, check(d), "gamelist.format", "xml")
}

func TestValidateRangeWithoutVersion(t *testing.T) {
	d := valid(t)
	d.Version.File = Fact{}
	d.Paths.Bios = Fact{Alts: []Fact{
		{Value: "{card}/BIOS", Src: "a", Until: "2"},
		{Value: "{card}/Bios", Src: "b", Since: "2"},
	}}
	wantErr(t, check(d), "paths.bios", "[version]")
}

func TestValidateRowRangeWithoutVersion(t *testing.T) {
	d := valid(t)
	d.Version.File = Fact{}
	d.Platforms["gba"] = Row{Roms: []string{"GBA"}, Saves: []string{"GBA"}, Since: "2"}
	wantErr(t, check(d), "platforms.gba", "[version]")
}

func TestValidateRangeWithVersionHook(t *testing.T) {
	d := valid(t)
	d.Version.File = Fact{}
	d.Version.Hook = "fw_version"
	d.Platforms["gba"] = Row{Roms: []string{"GBA"}, Saves: []string{"GBA"}, Since: "2"}
	reg := Registry{Version: map[string]VersionHook{"fw_version": nil}}
	if errs := Validate(d, knownForTest, reg, fstest.MapFS{}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateCardRange(t *testing.T) {
	d := valid(t)
	d.Paths.Card.Since = "2"
	wantErr(t, check(d), "paths.card", "range")

	d = valid(t)
	d.Paths.Card = Fact{Alts: []Fact{{Value: "/a", Src: "x", Until: "2"}, {Value: "/b", Src: "x", Since: "2"}}}
	wantErr(t, check(d), "paths.card", "range")
}

func TestValidateVersionFileRange(t *testing.T) {
	d := valid(t)
	d.Version.File.Until = "9"
	wantErr(t, check(d), "version.file", "range")
}

func TestValidateAlternativesOverlap(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Alts: []Fact{
		{Value: "{card}/BIOS", Src: "a", Until: "3"},
		{Value: "{card}/Bios", Src: "b", Since: "2"},
	}}
	wantErr(t, check(d), "paths.bios", "overlap")
}

func TestValidateAlternativesAdjacentAllowed(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Alts: []Fact{
		{Value: "{card}/BIOS", Src: "a", Until: "2"},
		{Value: "{card}/Bios", Src: "b", Since: "2"},
	}}
	if errs := check(d); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateAlternativesOpenBoth(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Alts: []Fact{
		{Value: "{card}/BIOS", Src: "a"},
		{Value: "{card}/Bios", Src: "b", Since: "2"},
	}}
	wantErr(t, check(d), "paths.bios", "overlap")
}

func TestValidateEmptyRange(t *testing.T) {
	d := valid(t)
	d.Paths.Bios = Fact{Value: "{card}/Bios", Src: "a", Since: "3", Until: "2"}
	wantErr(t, check(d), "paths.bios", "range")
}

func TestValidatePathsRequired(t *testing.T) {
	d := valid(t)
	d.Paths.Card = Fact{}
	d.Paths.Roms = Fact{}
	d.Paths.Bios = Fact{}
	d.Paths.Saves = Fact{}
	errs := check(d)
	wantErr(t, errs, "paths.card")
	wantErr(t, errs, "paths.roms")
	wantErr(t, errs, "paths.bios")
	wantErr(t, errs, "paths.saves")
}

func TestValidateSavesBesideRomsPath(t *testing.T) {
	d := valid(t)
	d.Paths.Saves = Fact{Value: "{roms}", Src: "x"}
	wantErr(t, check(d), "paths.saves", "beside-roms")

	d = valid(t)
	d.Paths.Saves = Fact{Value: "{roms}", Src: "x"}
	d.Saves.Layout = Fact{Value: "beside-roms", Src: "x"}
	d.PlatformSources = map[string]string{"roms": "sys/roms"}
	d.Platforms["gba"] = Row{Roms: []string{"GBA"}}
	if errs := check(d); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidatePlatformSources(t *testing.T) {
	d := valid(t)
	d.PlatformSources = map[string]string{"saves": "x"}
	wantErr(t, check(d), "platform_sources.roms")

	d = valid(t)
	d.PlatformSources = map[string]string{"roms": "x"}
	wantErr(t, check(d), "platform_sources.saves")

	d = valid(t)
	d.PlatformSources["extra"] = "x"
	wantErr(t, check(d), "platform_sources.extra")
}

func TestValidateBlankFirstRom(t *testing.T) {
	d := valid(t)
	d.Platforms["gba"] = Row{Roms: []string{"  ", "GBA"}, Saves: []string{"GBA"}}
	wantErr(t, check(d), "platforms.gba", "roms")
}

func TestValidateBesideRomsWithSaves(t *testing.T) {
	d := valid(t)
	d.Saves.Layout = Fact{Value: "beside-roms", Src: "x"}
	d.Paths.Saves = Fact{Value: "{roms}", Src: "x"}
	wantErr(t, check(d), "platforms.gba", "saves")
}

func TestValidateMissingInputFile(t *testing.T) {
	d := valid(t)
	d.Devices.Inputs = map[string]Input{"flip": {File: "flip.json"}}
	wantErr(t, check(d), "devices.inputs.flip", "flip.json")

	in := fstest.MapFS{"flip.json": &fstest.MapFile{Data: []byte("{}")}}
	if errs := Validate(d, knownForTest, Registry{}, in); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateInputNeedsFileOrHook(t *testing.T) {
	d := valid(t)
	d.Devices.Inputs = map[string]Input{"flip": {}}
	wantErr(t, check(d), "devices.inputs.flip", "file or a hook")
}

func TestValidateUnknownInputDevice(t *testing.T) {
	d := valid(t)
	d.Devices.Inputs = map[string]Input{"ghost": {File: "g.json"}}
	in := fstest.MapFS{"g.json": &fstest.MapFile{}}
	wantErr(t, Validate(d, knownForTest, Registry{}, in), "devices.inputs.ghost")
}

func TestValidateUnknownQuirkDevice(t *testing.T) {
	d := valid(t)
	d.Devices.Quirks["ghost"] = Quirks{Rotation: 90}
	wantErr(t, check(d), "devices.quirks.ghost")
}

func TestValidateDefaultDeviceAllowed(t *testing.T) {
	d := valid(t)
	d.Devices.Quirks["generic"] = Quirks{Rotation: 90}
	if errs := check(d); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateDeviceRuleProbe(t *testing.T) {
	d := valid(t)
	d.Devices.Rules = append(d.Devices.Rules, DeviceRule{Device: "a", Env: "X"})
	wantErr(t, check(d), "devices.rules[2]")

	d = valid(t)
	d.Devices.Rules = append(d.Devices.Rules, DeviceRule{Env: "X", Equals: "1"})
	wantErr(t, check(d), "devices.rules[2]", "device")

	d = valid(t)
	d.Devices.Rules = append(d.Devices.Rules, DeviceRule{Device: "a", Env: "X", Equals: "1", File: "/f", Contains: "c"})
	wantErr(t, check(d), "devices.rules[2]")
}

func TestValidateVersionFileAndHook(t *testing.T) {
	d := valid(t)
	d.Version.Hook = "fw_version"
	reg := Registry{Version: map[string]VersionHook{"fw_version": nil}}
	wantErr(t, Validate(d, knownForTest, reg, fstest.MapFS{}), "version", "file", "hook")
}

func TestValidateVersionFileHook(t *testing.T) {
	d := valid(t)
	d.Version.File = Fact{Hook: "fw_v"}
	wantErr(t, check(d), "version.file", "hook")
}

func TestValidatePackaging(t *testing.T) {
	d := valid(t)
	d.Packaging = Packaging{}
	errs := check(d)
	wantErr(t, errs, "packaging.asset")
	wantErr(t, errs, "packaging.launch_script")
	wantErr(t, errs, "packaging.install_depth")
}

func TestValidateRetroArch(t *testing.T) {
	d := valid(t)
	d.RetroArch = &struct {
		Config    *Fact `toml:"config"`
		Overrides *Fact `toml:"overrides"`
	}{Config: &Fact{Value: "{card}/cfg", Src: "x"}, Overrides: &Fact{Value: "{bios}/ov", Src: "x"}}
	if errs := check(d); len(errs) != 0 {
		t.Fatal(errs)
	}

	d.RetroArch.Overrides = &Fact{Value: "{rom_dir}/ov", Src: "x"}
	wantErr(t, check(d), "retroarch.overrides", "rom_dir")

	d.RetroArch.Config = nil
	wantErr(t, check(d), "retroarch.config")
}

func TestValidateCollectsAllErrors(t *testing.T) {
	d := valid(t)
	d.Name = ""
	d.Maintained = "vendor"
	d.Paths.Roms = Fact{Value: "{nope}", Src: "x"}
	d.Platforms["bad"] = Row{}
	d.Devices.Quirks["ghost"] = Quirks{}
	errs := check(d)
	if len(errs) < 5 {
		t.Fatalf("want at least 5 errors, got %v", errs)
	}
}

func TestValidateAllowsEnvVariablesInPaths(t *testing.T) {
	d := valid(t)
	d.Paths.Card = Fact{Value: "{env.ROOT}", Src: "sys/a"}
	for _, e := range check(d) {
		if strings.Contains(e.Error(), "template variable") {
			t.Fatal(e)
		}
	}
}

func TestValidateHostSkipsPackagingAndInputFiles(t *testing.T) {
	d := valid(t)
	d.Packaging = Packaging{}
	d.Devices.Inputs = map[string]Input{"trimui": {File: "missing.txt"}}
	errs := check(d)
	wantErr(t, errs, "packaging.asset")
	wantErr(t, errs, "input file")
	if errs := ValidateHost(d, knownForTest, Registry{}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateHostKeepsSrcRule(t *testing.T) {
	d := valid(t)
	d.Paths.Roms.Src = ""
	wantErr(t, ValidateHost(d, knownForTest, Registry{}), "paths.roms", "src")
}

func TestMultiLayoutHookValidated(t *testing.T) {
	d := valid(t)
	d.RomFolders.MultiLayout = "fw_layout"
	wantErr(t, check(d), "rom_folders.multi_layout", "fw_layout")
	reg := Registry{Layout: map[string]LayoutHook{"fw_layout": nil}}
	if errs := Validate(d, knownForTest, reg, fstest.MapFS{}); len(errs) != 0 {
		t.Fatal(errs)
	}
}

func TestValidateNewArtKinds(t *testing.T) {
	d := valid(t)
	d.Art.Thumbnail = &Fact{Value: "{rom_dir}/images"}
	d.Art.Fanart = &Fact{Value: "{nope}/images", Src: "x"}
	errs := check(d)
	wantErr(t, errs, "art.thumbnail", "src is required")
	wantErr(t, errs, "art.fanart", "{nope}")
}

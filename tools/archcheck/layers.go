package main

import "strings"

// Layer is one horizontal band of the architecture. A package may only import
// packages in the layers its own layer permits.
type Layer string

const (
	// Pkg holds standalone utilities with no grout dependencies beyond other
	// pkg packages: archives, hashing, images, text.
	Pkg Layer = "pkg"
	// Domain holds the types and rules grout works in: games, platforms,
	// settings, sync decisions. No I/O, no device, no network.
	Domain Layer = "domain"
	// Platform knows about firmware: where things live on device, what a
	// gamelist looks like, how saves are laid out.
	Platform Layer = "platform"
	// Infra talks to the outside world: the RomM API, the sqlite store, the
	// settings file.
	Infra Layer = "infra"
	// Service orchestrates a use case across the layers below it.
	Service Layer = "service"
	// UI draws screens and reads input. It may call services and speak in
	// domain types, but must not reach infrastructure or the device directly.
	UI Layer = "ui"
	// Cmd is the composition root. It is the only place allowed to wire
	// everything together, hold process state, or exit.
	Cmd Layer = "cmd"
)

// allowedImports is a staircase: each layer may use everything below it. UI is
// the exception, reaching services and domain but not infrastructure or the
// device, so a screen cannot open a database or build a firmware path.
var allowedImports = map[Layer]map[Layer]bool{
	Pkg:      set(Pkg),
	Domain:   set(Pkg, Domain),
	Platform: set(Pkg, Domain, Platform),
	Infra:    set(Pkg, Domain, Platform, Infra),
	Service:  set(Pkg, Domain, Platform, Infra, Service),
	UI:       set(Pkg, Domain, Service, UI),
	Cmd:      set(Pkg, Domain, Platform, Infra, Service, UI, Cmd),
}

// layerRules maps an import path to its layer; the longest matching prefix
// wins. Domain packages are listed individually rather than sharing a prefix,
// so adding one is a deliberate act.
var layerRules = []struct {
	prefix string
	layer  Layer
}{
	{"grout/library", Domain},
	{"grout/settings", Domain},

	{"grout/cfw", Platform},
	{"grout/gamelist", Platform},

	{"grout/romm", Infra},
	{"grout/cache", Infra},

	{"grout/auth", Service},
	{"grout/catalog", Service},
	{"grout/download", Service},
	{"grout/saves", Service},
	{"grout/bios", Service},
	{"grout/update", Service},

	{"grout/ui", UI},
	{"grout/app", Cmd},

	// Leaf utilities.
	{"grout/archive", Pkg},
	{"grout/environment", Pkg},
	{"grout/files", Pkg},
	{"grout/hashing", Pkg},
	{"grout/imaging", Pkg},
	{"grout/pspdb", Pkg},
	{"grout/resources", Pkg},
	{"grout/tables", Pkg},
	{"grout/textmatch", Pkg},
	{"grout/version", Pkg},
}

// layerOf returns the layer for an import path, and whether it is a grout
// package at all.
func layerOf(importPath string) (Layer, bool) {
	best := ""
	var layer Layer
	for _, rule := range layerRules {
		if importPath != rule.prefix && !strings.HasPrefix(importPath, rule.prefix+"/") {
			continue
		}
		if len(rule.prefix) > len(best) {
			best, layer = rule.prefix, rule.layer
		}
	}
	if best == "" {
		return "", false
	}
	return layer, true
}

// isExempt excludes developer tools, which never ship to a device.
func isExempt(importPath string) bool {
	return strings.HasPrefix(importPath, "grout/tools/")
}

// toolkitPrefix is the UI toolkit. Importing it links SDL, so the package
// cannot be tested without a display.
const toolkitPrefix = "github.com/BrandonKowalski/gabagool/v2"

func mayUseToolkit(l Layer) bool { return l == UI || l == Cmd }

// mayHoldGlobalState is true only for the composition root.
func mayHoldGlobalState(l Layer) bool { return l == Cmd }

func set(layers ...Layer) map[Layer]bool {
	m := make(map[Layer]bool, len(layers))
	for _, l := range layers {
		m[l] = true
	}
	return m
}

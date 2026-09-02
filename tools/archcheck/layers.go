package main

import "strings"

// Layer is one horizontal band of the architecture. A package may only import
// packages in the layers its own layer permits.
type Layer string

const (
	// Pkg holds standalone utilities with no grout dependencies beyond other
	// pkg packages: archives, hashing, images, text.
	Pkg Layer = "pkg"
	// Domain holds the types and rules grout works in -- games, platforms,
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

// allowedImports records, for each layer, the layers it may import.
//
// The shape is a staircase: each layer may use everything below it. UI is the
// exception -- it may reach services and domain types but not infrastructure or
// the device, so a screen cannot open a database or build a firmware path.
var allowedImports = map[Layer]map[Layer]bool{
	Pkg:      set(Pkg),
	Domain:   set(Pkg, Domain),
	Platform: set(Pkg, Domain, Platform),
	Infra:    set(Pkg, Domain, Platform, Infra),
	Service:  set(Pkg, Domain, Platform, Infra, Service),
	UI:       set(Pkg, Domain, Service, UI),
	Cmd:      set(Pkg, Domain, Platform, Infra, Service, UI, Cmd),
}

// layerOf maps an import path to its layer. The longest matching prefix wins,
// so a specific subpackage can differ from its parent.
//
// Several packages are classified by where they are going rather than where
// they are: grout/internal is the settings god-object whose destination is the
// domain, so its imports of cache and cfw show up as violations. That is
// intentional -- allow.txt carries them until the split lands.
var layerRules = []struct {
	prefix string
	layer  Layer
}{
	{"grout/domain", Domain},

	{"grout/cfw", Platform},
	{"grout/internal/gamelist", Platform},
	{"grout/internal/emulationstation", Platform},

	{"grout/romm", Infra},
	{"grout/cache", Infra},

	{"grout/sync", Service},
	{"grout/bios", Service},
	{"grout/update", Service},

	{"grout/ui", UI},
	{"grout/app", Cmd},

	// Leaf utilities. These are the pkg/ tree in waiting; they already have no
	// grout dependencies, so classifying them now costs nothing.
	{"grout/internal/artutil", Pkg},
	{"grout/internal/environment", Pkg},
	{"grout/internal/fileutil", Pkg},
	{"grout/internal/imageutil", Pkg},
	{"grout/internal/jsonutil", Pkg},
	{"grout/internal/pspdb", Pkg},
	{"grout/internal/stringutil", Pkg},
	{"grout/resources", Pkg},
	{"grout/version", Pkg},

	// The settings god-object. Target is a leaf domain package.
	{"grout/internal", Domain},
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

// isExempt reports whether a package is outside archcheck's remit. Developer
// tools are standalone programs that never ship to a device.
func isExempt(importPath string) bool {
	return strings.HasPrefix(importPath, "grout/tools/")
}

// toolkitPrefix is the UI toolkit. Importing it links SDL, which is why a
// package that pulls it in cannot be tested without a display.
const toolkitPrefix = "github.com/BrandonKowalski/gabagool/v2"

// mayUseToolkit reports whether a layer is allowed to import the UI toolkit.
func mayUseToolkit(l Layer) bool { return l == UI || l == Cmd }

// mayHoldGlobalState reports whether a layer may declare package-level mutable
// state. Only the composition root may.
func mayHoldGlobalState(l Layer) bool { return l == Cmd }

func set(layers ...Layer) map[Layer]bool {
	m := make(map[Layer]bool, len(layers))
	for _, l := range layers {
		m[l] = true
	}
	return m
}

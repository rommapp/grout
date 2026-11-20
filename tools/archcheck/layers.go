package main

import "strings"

// Layer is one horizontal band of the architecture. A package may only import
// packages in the layers its own layer permits.
type Layer string

const (
	Pkg  Layer = "pkg"
	Core Layer = "core"
	Lib  Layer = "lib"
	API  Layer = "api"
	CAPI Layer = "capi"
	UI   Layer = "ui"
	CLI  Layer = "cli"
	Cmd  Layer = "cmd"
)

var allowedImports = map[Layer]map[Layer]bool{
	Pkg:  set(Pkg),
	Core: set(Pkg, Core),
	Lib:  set(Pkg, Core),
	API:  set(Pkg, Core, Lib),
	CAPI: set(Pkg, API),
	UI:   set(Pkg, Core, Lib, UI),
	CLI:  set(Pkg, API, CLI),
	Cmd:  set(Pkg, Core, Lib, API, CAPI, UI, CLI, Cmd),
}

var layerRules = []struct {
	prefix string
	layer  Layer
	exact  bool
}{
	{"github.com/rommapp/grout", Lib, true},
	{"github.com/rommapp/grout/api", API, false},
	{"github.com/rommapp/grout/capi", CAPI, false},
	{"github.com/rommapp/grout/core", Core, false},
	{"github.com/rommapp/grout/ui", UI, false},
	{"github.com/rommapp/grout/cli", CLI, false},
	{"github.com/rommapp/grout/cmd", Cmd, false},
	{"github.com/rommapp/grout/resources", UI, false},
	{"github.com/rommapp/grout/version", Pkg, false},
}

// layerOf returns the layer for an import path, and whether it is a grout
// package at all.
func layerOf(importPath string) (Layer, bool) {
	best := ""
	var layer Layer
	for _, rule := range layerRules {
		if importPath != rule.prefix && (rule.exact || !strings.HasPrefix(importPath, rule.prefix+"/")) {
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

// isExempt excludes developer tools and test helpers, which never ship to a
// device. The e2e keyboard helper is Linux-only, so it only showed up here on
// CI.
func isExempt(importPath string) bool {
	return strings.HasPrefix(importPath, "github.com/rommapp/grout/tools/") || strings.HasPrefix(importPath, "github.com/rommapp/grout/test/")
}

// toolkitPrefix is the UI toolkit. Importing it links SDL, so the package
// cannot be tested without a display.
const toolkitPrefix = "github.com/BrandonKowalski/gabagool/v2"

func mayUseToolkit(l Layer) bool { return l == UI || l == Cmd }

func mayHoldGlobalState(l Layer) bool { return l == Cmd || l == CAPI }

func checksProcess(l Layer) bool { return l == Core || l == Lib || l == API || l == CAPI }

func set(layers ...Layer) map[Layer]bool {
	m := make(map[Layer]bool, len(layers))
	for _, l := range layers {
		m[l] = true
	}
	return m
}

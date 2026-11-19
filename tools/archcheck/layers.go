package main

import "strings"

// Layer is one horizontal band of the architecture. A package may only import
// packages in the layers its own layer permits.
type Layer string

const (
	Pkg  Layer = "pkg"
	Core Layer = "core"
	UI   Layer = "ui"
	CLI  Layer = "cli"
	Cmd  Layer = "cmd"
)

var allowedImports = map[Layer]map[Layer]bool{
	Pkg:  set(Pkg),
	Core: set(Pkg, Core),
	UI:   set(Pkg, Core, UI),
	CLI:  set(Pkg, Core, CLI),
	Cmd:  set(Pkg, Core, UI, CLI, Cmd),
}

var layerRules = []struct {
	prefix string
	layer  Layer
}{
	{"grout/core", Core},
	{"grout/ui", UI},
	{"grout/cli", CLI},
	{"grout/cmd", Cmd},
	{"grout/resources", UI},
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

// isExempt excludes developer tools and test helpers, which never ship to a
// device. The e2e keyboard helper is Linux-only, so it only showed up here on
// CI.
func isExempt(importPath string) bool {
	return strings.HasPrefix(importPath, "grout/tools/") || strings.HasPrefix(importPath, "grout/test/")
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

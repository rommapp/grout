package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLayerOf(t *testing.T) {
	tests := []struct {
		path string
		want Layer
		ok   bool
	}{
		{"grout/domain/library", Domain, true},
		{"grout/cfw", Platform, true},
		{"grout/cfw/muos", Platform, true},
		{"grout/romm", Infra, true},
		{"grout/cache", Infra, true},
		{"grout/sync", Service, true},
		{"grout/ui", UI, true},
		{"grout/app", Cmd, true},

		// The longest matching prefix wins, so a subpackage may differ from
		// its parent. internal is the settings god-object heading for the
		// domain; its leaf utilities are already pkg material.
		{"grout/internal", Domain, true},
		{"grout/internal/fileutil", Pkg, true},
		{"grout/internal/stringutil", Pkg, true},
		{"grout/internal/gamelist", Platform, true},

		// A prefix must not match a longer package name by accident.
		{"grout/uixyz", "", false},
		{"grout/syncthing", "", false},

		{"net/http", "", false},
		{"github.com/beevik/etree", "", false},
	}

	for _, tt := range tests {
		got, ok := layerOf(tt.path)
		if ok != tt.ok {
			t.Errorf("layerOf(%q) ok = %v, want %v", tt.path, ok, tt.ok)
			continue
		}
		if ok && got != tt.want {
			t.Errorf("layerOf(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// The matrix must be a staircase: each layer may import everything below it,
// which keeps the direction unambiguous. UI is the deliberate exception.
func TestAllowedImports_IsAStaircase(t *testing.T) {
	order := []Layer{Pkg, Domain, Platform, Infra, Service}

	for i, from := range order {
		for j, to := range order {
			allowed := allowedImports[from][to]
			want := j <= i
			if allowed != want {
				t.Errorf("%s importing %s = %v, want %v", from, to, allowed, want)
			}
		}
	}

	// UI reaches services and the domain but never infrastructure or the
	// device, so a screen cannot open a database or build a firmware path.
	for _, forbidden := range []Layer{Platform, Infra} {
		if allowedImports[UI][forbidden] {
			t.Errorf("ui must not be allowed to import %s", forbidden)
		}
	}
	for _, permitted := range []Layer{Pkg, Domain, Service, UI} {
		if !allowedImports[UI][permitted] {
			t.Errorf("ui should be allowed to import %s", permitted)
		}
	}

	// Nothing may import cmd, and cmd may import anything.
	for from := range allowedImports {
		if from != Cmd && allowedImports[from][Cmd] {
			t.Errorf("%s must not be allowed to import cmd", from)
		}
	}
	for _, to := range []Layer{Pkg, Domain, Platform, Infra, Service, UI, Cmd} {
		if !allowedImports[Cmd][to] {
			t.Errorf("cmd should be allowed to import %s", to)
		}
	}
}

func TestMayUseToolkitAndHoldState(t *testing.T) {
	for _, l := range []Layer{Pkg, Domain, Platform, Infra, Service} {
		if mayUseToolkit(l) {
			t.Errorf("%s must not be allowed the UI toolkit", l)
		}
		if mayHoldGlobalState(l) {
			t.Errorf("%s must not be allowed package-level state", l)
		}
	}
	if !mayUseToolkit(UI) || !mayUseToolkit(Cmd) {
		t.Error("ui and cmd may use the toolkit")
	}
	if mayHoldGlobalState(UI) {
		t.Error("only cmd may hold package-level state")
	}
	if !mayHoldGlobalState(Cmd) {
		t.Error("cmd may hold package-level state")
	}
}

// parseSrc turns a snippet into the file list the checks expect.
func parseSrc(t *testing.T, src string) []*ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return []*ast.File{f}
}

func mutatedIn(t *testing.T, src string) []string {
	t.Helper()
	files := parseSrc(t, src)
	m := mutatedNames(files, packageVars(files))
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}

// The point of the globals rule is state that changes at runtime. Declaring a
// value once is not that, or the rule would flag every embedded file and error
// sentinel in the repo and be ignored.
func TestMutatedNames_IgnoresWriteOnceDeclarations(t *testing.T) {
	src := `package p

import (
	"embed"
	"errors"
	"regexp"
)

//go:embed data
var embedded embed.FS

var ErrThing = errors.New("thing")

var re = regexp.MustCompile("x")

var lookup = map[string]string{"a": "b"}

func read() string { return lookup["a"] }
`
	if got := mutatedIn(t, src); len(got) != 0 {
		t.Errorf("mutated = %v, want none", got)
	}
}

func TestMutatedNames_FindsRuntimeWrites(t *testing.T) {
	src := `package p

var counter int
var manager *Thing
var name string

type Thing struct{}

func set() {
	manager = &Thing{}
	counter++
	name = "x"
}
`
	want := []string{"counter", "manager", "name"}
	got := mutatedIn(t, src)
	if !slices.Equal(got, want) {
		t.Errorf("mutated = %v, want %v", got, want)
	}
}

// Writing to a field or key of a package-level var changes process state just
// as surely as reassigning it. ui/status_bar.go is the case that motivated
// this: AddStatusBarIcon appends to a field rather than to the var.
func TestMutatedNames_FindsFieldAndKeyWrites(t *testing.T) {
	src := `package p

type Bar struct{ Icons []string }

var status Bar
var table = map[string]int{}

func add(s string) {
	status.Icons = append(status.Icons, s)
	table["k"] = 1
}
`
	want := []string{"status", "table"}
	got := mutatedIn(t, src)
	if !slices.Equal(got, want) {
		t.Errorf("mutated = %v, want %v", got, want)
	}
}

// A value mutated only through methods has no assignment to find, so its type
// is what gives it away.
func TestMutatedNames_FindsMethodMutatedTypes(t *testing.T) {
	src := `package p

import (
	"sync"

	"go.uber.org/atomic"
)

var enabled atomic.Bool
var cache sync.Map
var guard sync.Mutex

func set() { enabled.Store(true) }
`
	got := mutatedIn(t, src)
	if !slices.Contains(got, "enabled") {
		t.Errorf("mutated = %v, want it to include the atomic value", got)
	}
	if !slices.Contains(got, "cache") {
		t.Errorf("mutated = %v, want it to include the sync.Map", got)
	}
	// A mutex is a guard for other state, not state anyone reads. Flagging it
	// would just add noise beside the value it protects.
	if slices.Contains(got, "guard") {
		t.Errorf("mutated = %v, should not include a plain mutex", got)
	}
}

// Locals that shadow nothing must not be mistaken for package state.
func TestMutatedNames_IgnoresLocals(t *testing.T) {
	src := `package p

func f() {
	local := 1
	local++
	other := &local
	_ = other
}
`
	if got := mutatedIn(t, src); len(got) != 0 {
		t.Errorf("mutated = %v, want none", got)
	}
}

func TestRootIdent(t *testing.T) {
	tests := map[string]string{
		"x":          "x",
		"x.y":        "x",
		"x.y.z":      "x",
		"x[0]":       "x",
		"x.y[0].z":   "x",
		"(*x).y":     "x",
		"f().y":      "",
		"pkg.Fn().x": "",
	}
	for src, want := range tests {
		expr, err := parser.ParseExpr(src)
		if err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
		if got := rootIdent(expr); got != want {
			t.Errorf("rootIdent(%q) = %q, want %q", src, got, want)
		}
	}
}

// A package can break more than one rule at once. Reporting the toolkit import
// must not stop the layer scan, or every gabagool-importing package (most of
// the repo) goes unchecked.
func TestCheckImports_ReportsToolkitAndLayerTogether(t *testing.T) {
	p := pkg{
		ImportPath: "grout/cache",
		Imports: []string{
			toolkitPrefix + "/pkg/gabagool", // sorts before grout/... as go list emits it
			"grout/ui",
		},
	}

	var rules []string
	for _, v := range checkImports(p, Infra) {
		rules = append(rules, v.Rule)
	}
	slices.Sort(rules)

	if !slices.Equal(rules, []string{"layer", "toolkit"}) {
		t.Errorf("rules = %v, want both a layer and a toolkit violation", rules)
	}
}

func TestCheckImports_AllowsPermittedEdges(t *testing.T) {
	p := pkg{
		ImportPath: "grout/sync",
		Imports:    []string{"grout/cache", "grout/domain/library", "grout/cfw", "net/http"},
	}
	if got := checkImports(p, Service); len(got) != 0 {
		t.Errorf("expected no violations for a service importing infra, got %v", got)
	}
}

// The allowlist key must not carry anything that changes when unrelated code
// moves, or the file would churn and stop being reviewed.
func TestViolationKey_IsStable(t *testing.T) {
	a := violation{Rule: "layer", Subject: "grout/ui -> grout/cache", Detail: "one wording", Files: []string{"a.go"}}
	b := violation{Rule: "layer", Subject: "grout/ui -> grout/cache", Detail: "another wording", Files: []string{"b.go", "c.go"}}

	if a.Key() != b.Key() {
		t.Errorf("keys differ despite the same rule and subject: %q vs %q", a.Key(), b.Key())
	}
	if !strings.HasPrefix(a.Key(), "layer ") {
		t.Errorf("key %q should start with its rule", a.Key())
	}
}

func TestAllowFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	defer func() { _ = os.Chdir(cwd) }()

	if err := os.MkdirAll(filepath.Dir(allowFile), 0755); err != nil {
		t.Fatal(err)
	}

	want := []violation{
		{Rule: "layer", Subject: "grout/ui -> grout/cache"},
		{Rule: "toolkit", Subject: "grout/romm"},
		{Rule: "global", Subject: "grout/cache.cacheManager"},
	}
	if err := writeAllowFile(want); err != nil {
		t.Fatalf("writeAllowFile: %v", err)
	}

	got, err := readAllowFile()
	if err != nil {
		t.Fatalf("readAllowFile: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("read back %d entries, want %d", len(got), len(want))
	}
	for _, v := range want {
		if !got[v.Key()] {
			t.Errorf("entry %q did not survive the round trip", v.Key())
		}
	}
}

func TestReadAllowFile_SkipsCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.MkdirAll(filepath.Dir(allowFile), 0755); err != nil {
		t.Fatal(err)
	}
	content := "# a comment\n\nlayer grout/ui -> grout/cache\n\n  # indented comment\ntoolkit grout/romm\n"
	if err := os.WriteFile(allowFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readAllowFile()
	if err != nil {
		t.Fatalf("readAllowFile: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("read %d entries, want 2: %v", len(got), got)
	}
}

// A missing allowlist means nothing is tolerated, not that everything is.
func TestReadAllowFile_MissingFileIsEmpty(t *testing.T) {
	t.Chdir(t.TempDir())

	got, err := readAllowFile()
	if err != nil {
		t.Fatalf("readAllowFile: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d entries for a missing file, want 0", len(got))
	}
}

func TestIsExempt(t *testing.T) {
	if !isExempt("grout/tools/archcheck") {
		t.Error("developer tools should be exempt")
	}
	if isExempt("grout/ui") {
		t.Error("grout/ui must not be exempt")
	}
}

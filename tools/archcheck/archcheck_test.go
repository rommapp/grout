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
	cases := map[string]Layer{
		"grout/core/config":        Core,
		"grout/core/firmware/muos": Core,
		"grout/ui":                 UI,
		"grout/cli":                CLI,
		"grout/cmd/grout":          Cmd,
		"grout/version":            Pkg,
		"grout/resources":          UI,
	}
	for path, want := range cases {
		got, ok := layerOf(path)
		if !ok || got != want {
			t.Errorf("layerOf(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
}

func TestLayerOfRejectsNonGroutAndNearMisses(t *testing.T) {
	for _, path := range []string{"grout/uixyz", "grout/clix", "grout/coreutil", "net/http", "github.com/beevik/etree"} {
		if l, ok := layerOf(path); ok {
			t.Errorf("layerOf(%q) = %q, want no layer", path, l)
		}
	}
}

func TestCoreMayNotImportFrontEnds(t *testing.T) {
	for _, target := range []Layer{UI, CLI, Cmd} {
		if allowedImports[Core][target] {
			t.Errorf("core may import %s", target)
		}
	}
}

func TestFrontEndsMayNotImportEachOther(t *testing.T) {
	if allowedImports[UI][CLI] || allowedImports[CLI][UI] {
		t.Error("ui and cli may import each other")
	}
}

func TestOnlyUIAndCmdUseToolkit(t *testing.T) {
	for _, l := range []Layer{Pkg, Core, CLI} {
		if mayUseToolkit(l) {
			t.Errorf("%s may use toolkit", l)
		}
	}
}

func TestOnlyCmdHoldsGlobalState(t *testing.T) {
	for _, l := range []Layer{Pkg, Core, UI, CLI} {
		if mayHoldGlobalState(l) {
			t.Errorf("%s may hold package-level state", l)
		}
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
// must not stop the layer scan.
func TestCheckImports_ReportsToolkitAndLayerTogether(t *testing.T) {
	p := pkg{
		ImportPath: "grout/core/store",
		Imports: []string{
			toolkitPrefix + "/pkg/gabagool", // sorts before grout/... as go list emits it
			"grout/ui",
		},
	}

	var rules []string
	for _, v := range checkImports(p, Core) {
		rules = append(rules, v.Rule)
	}
	slices.Sort(rules)

	if !slices.Equal(rules, []string{"layer", "toolkit"}) {
		t.Errorf("rules = %v, want both a layer and a toolkit violation", rules)
	}
}

func TestCheckImports_AllowsPermittedEdges(t *testing.T) {
	p := pkg{
		ImportPath: "grout/cli",
		Imports:    []string{"grout/core/config", "grout/core/romm", "grout/version", "net/http"},
	}
	if got := checkImports(p, CLI); len(got) != 0 {
		t.Errorf("expected no violations for cli importing core, got %v", got)
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
	if !isExempt("grout/test/e2e/groutkeys") {
		t.Error("test helpers never ship and should be exempt")
	}
	if !isExempt("grout/tools/archcheck") {
		t.Error("developer tools should be exempt")
	}
	if isExempt("grout/ui") {
		t.Error("grout/ui must not be exempt")
	}
}

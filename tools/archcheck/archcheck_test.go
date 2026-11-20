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
		"github.com/rommapp/grout/core/config":        Core,
		"github.com/rommapp/grout/core/firmware/muos": Core,
		"github.com/rommapp/grout/ui":                 UI,
		"github.com/rommapp/grout/cli":                CLI,
		"github.com/rommapp/grout/cmd/grout":          Cmd,
		"github.com/rommapp/grout/version":            Pkg,
		"github.com/rommapp/grout/resources":          UI,
	}
	for path, want := range cases {
		got, ok := layerOf(path)
		if !ok || got != want {
			t.Errorf("layerOf(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
}

func TestLayerOfRejectsNonGroutAndNearMisses(t *testing.T) {
	for _, path := range []string{"github.com/rommapp/grout/uixyz", "github.com/rommapp/grout/clix", "github.com/rommapp/grout/coreutil", "net/http", "github.com/beevik/etree"} {
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
		ImportPath: "github.com/rommapp/grout/core/store",
		Imports: []string{
			toolkitPrefix + "/pkg/gabagool", // sorts before grout/... as go list emits it
			"github.com/rommapp/grout/ui",
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
		ImportPath: "github.com/rommapp/grout/cli",
		Imports:    []string{"github.com/rommapp/grout/api", "github.com/rommapp/grout/version", "net/http"},
	}
	if got := checkImports(p, CLI); len(got) != 0 {
		t.Errorf("expected no violations for cli importing api, got %v", got)
	}
}

// The allowlist key must not carry anything that changes when unrelated code
// moves, or the file would churn and stop being reviewed.
func TestViolationKey_IsStable(t *testing.T) {
	a := violation{Rule: "layer", Subject: "github.com/rommapp/grout/ui -> grout/cache", Detail: "one wording", Files: []string{"a.go"}}
	b := violation{Rule: "layer", Subject: "github.com/rommapp/grout/ui -> grout/cache", Detail: "another wording", Files: []string{"b.go", "c.go"}}

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
		{Rule: "layer", Subject: "github.com/rommapp/grout/ui -> grout/cache"},
		{Rule: "toolkit", Subject: "github.com/rommapp/grout/romm"},
		{Rule: "global", Subject: "github.com/rommapp/grout/cache.cacheManager"},
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
	if !isExempt("github.com/rommapp/grout/test/e2e/groutkeys") {
		t.Error("test helpers never ship and should be exempt")
	}
	if !isExempt("github.com/rommapp/grout/tools/archcheck") {
		t.Error("developer tools should be exempt")
	}
	if isExempt("github.com/rommapp/grout/ui") {
		t.Error("github.com/rommapp/grout/ui must not be exempt")
	}
}

func TestLayerOfNewLayers(t *testing.T) {
	cases := map[string]Layer{
		"github.com/rommapp/grout":              Lib,
		"github.com/rommapp/grout/api":          API,
		"github.com/rommapp/grout/capi":         CAPI,
		"github.com/rommapp/grout/core/library": Core,
		"github.com/rommapp/grout/cli":          CLI,
	}
	for path, want := range cases {
		got, ok := layerOf(path)
		if !ok || got != want {
			t.Errorf("layerOf(%q) = %q, %v; want %q", path, got, ok, want)
		}
	}
}

func TestLibIsMatchedExactly(t *testing.T) {
	if l, ok := layerOf("github.com/rommapp/grout"); !ok || l != Lib {
		t.Errorf("root = %q, %v", l, ok)
	}
	if l, ok := layerOf("github.com/rommapp/grout/nothing"); ok {
		t.Errorf("unknown subpackage classified as %q", l)
	}
}

func TestImportMatrix(t *testing.T) {
	want := map[Layer][]Layer{
		Pkg:  {Pkg},
		Core: {Pkg, Core},
		Lib:  {Pkg, Core},
		API:  {Pkg, Core, Lib},
		CAPI: {Pkg, API},
		UI:   {Pkg, Core, Lib, UI},
		CLI:  {Pkg, API, CLI},
		Cmd:  {Pkg, Core, Lib, API, CAPI, UI, CLI, Cmd},
	}
	all := []Layer{Pkg, Core, Lib, API, CAPI, UI, CLI, Cmd}
	for from, allowed := range want {
		for _, to := range all {
			if got := allowedImports[from][to]; got != slices.Contains(allowed, to) {
				t.Errorf("%s importing %s = %v", from, to, got)
			}
		}
	}
}

func TestOnlyCmdAndCAPIHoldGlobalState(t *testing.T) {
	for _, l := range []Layer{Pkg, Core, Lib, API, UI, CLI} {
		if mayHoldGlobalState(l) {
			t.Errorf("%s may hold package-level state", l)
		}
	}
	for _, l := range []Layer{Cmd, CAPI} {
		if !mayHoldGlobalState(l) {
			t.Errorf("%s may not hold package-level state", l)
		}
	}
}

func TestProcessRuleFlagsEachForbiddenUse(t *testing.T) {
	cases := map[string]string{
		"os.Exit":                `import "os"; func f() { os.Exit(1) }`,
		"os.Getenv":              `import "os"; func f() { _ = os.Getenv("A") }`,
		"os.LookupEnv":           `import "os"; func f() { os.LookupEnv("A") }`,
		"os.Environ":             `import "os"; func f() { _ = os.Environ() }`,
		"os.Setenv":              `import "os"; func f() { os.Setenv("A", "b") }`,
		"os.Unsetenv":            `import "os"; func f() { os.Unsetenv("A") }`,
		"os.Clearenv":            `import "os"; func f() { os.Clearenv() }`,
		"os.Chdir":               `import "os"; func f() { os.Chdir("/") }`,
		"os.Stdin":               `import "os"; var _ = os.Stdin`,
		"os.Stdout":              `import "os"; var _ = os.Stdout`,
		"os.Stderr":              `import "os"; var _ = os.Stderr`,
		"log.Println":            `import "log"; func f() { log.Println("x") }`,
		"log.Fatal":              `import "log"; func f() { log.Fatal("x") }`,
		"fmt.Print":              `import "fmt"; func f() { fmt.Print("x") }`,
		"fmt.Printf":             `import "fmt"; func f() { fmt.Printf("x") }`,
		"fmt.Println":            `import "fmt"; func f() { fmt.Println("x") }`,
		"slog.Debug":             `import "log/slog"; func f() { slog.Debug("x") }`,
		"slog.Info":              `import "log/slog"; func f() { slog.Info("x") }`,
		"slog.Warn":              `import "log/slog"; func f() { slog.Warn("x") }`,
		"slog.Error":             `import "log/slog"; func f() { slog.Error("x") }`,
		"slog.Log":               `import ("context"; "log/slog"); func f() { slog.Log(context.TODO(), slog.LevelInfo, "x") }`,
		"slog.LogAttrs":          `import ("context"; "log/slog"); func f() { slog.LogAttrs(context.TODO(), slog.LevelInfo, "x") }`,
		"slog.DebugContext":      `import ("context"; "log/slog"); func f() { slog.DebugContext(context.TODO(), "x") }`,
		"slog.InfoContext":       `import ("context"; "log/slog"); func f() { slog.InfoContext(context.TODO(), "x") }`,
		"slog.WarnContext":       `import ("context"; "log/slog"); func f() { slog.WarnContext(context.TODO(), "x") }`,
		"slog.ErrorContext":      `import ("context"; "log/slog"); func f() { slog.ErrorContext(context.TODO(), "x") }`,
		"slog.Default":           `import "log/slog"; func f() { _ = slog.Default() }`,
		"slog.SetDefault":        `import "log/slog"; func f() { slog.SetDefault(nil) }`,
		"slog.SetLogLoggerLevel": `import "log/slog"; func f() { slog.SetLogLoggerLevel(slog.LevelInfo) }`,
		"signal.Notify":          `import "os/signal"; func f() { signal.Notify(nil) }`,
		"signal.Ignore":          `import "os/signal"; func f() { signal.Ignore() }`,
		"signal.Reset":           `import "os/signal"; func f() { signal.Reset() }`,
		"flag.Parse":             `import "flag"; func f() { flag.Parse() }`,
		"flag.String":            `import "flag"; func f() { flag.String("a", "", "") }`,
	}
	for want, body := range cases {
		got := processUses(parseSrc(t, "package x\n"+body))
		if !slices.Equal(got, []string{want}) {
			t.Errorf("%s: uses = %v", want, got)
		}
	}
}

func TestProcessRuleAllowsInjectedUse(t *testing.T) {
	src := `package x
import ("fmt"; "log/slog"; "os")
func f(l *slog.Logger) {
	l.Warn("x")
	_ = fmt.Sprintf("x")
	_ = os.ReadFile
	_ = slog.LevelInfo
}`
	if got := processUses(parseSrc(t, src)); len(got) != 0 {
		t.Errorf("uses = %v", got)
	}
}

func TestProcessRuleResolvesImportAliases(t *testing.T) {
	aliased := `package x
import l "log"
func f() { l.Println("x") }`
	if got := processUses(parseSrc(t, aliased)); !slices.Equal(got, []string{"log.Println"}) {
		t.Errorf("aliased uses = %v", got)
	}
	local := `package x
import "os"
var _ = os.Getpid
type exiter struct{}
func (exiter) Exit() {}
func f() {
	os := exiter{}
	os.Exit()
}`
	if got := processUses(parseSrc(t, local)); len(got) != 0 {
		t.Errorf("local variable uses = %v", got)
	}
	param := `package x
import "fmt"
type printer struct{}
func (printer) Println(...any) {}
func f(fmt printer) { fmt.Println(1) }`
	if got := processUses(parseSrc(t, param)); len(got) != 0 {
		t.Errorf("parameter uses = %v", got)
	}
}

func TestProcessRuleChecksCgoFiles(t *testing.T) {
	dir := t.TempDir()
	src := "package x\nimport \"os\"\nfunc f() { os.Exit(1) }\n"
	if err := os.WriteFile(filepath.Join(dir, "c.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p := pkg{ImportPath: "github.com/rommapp/grout/capi", Dir: dir, CgoFiles: []string{"c.go"}}
	got := checkProcess(p, CAPI)
	if len(got) != 1 || got[0].Subject != p.ImportPath+" uses os.Exit" {
		t.Errorf("got %v", got)
	}
}

func TestProcessRuleIgnoresCLIAndCmd(t *testing.T) {
	dir := t.TempDir()
	src := "package x\nimport \"fmt\"\nfunc f() { fmt.Println(1) }\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	p := pkg{ImportPath: "github.com/rommapp/grout/cli", Dir: dir, GoFiles: []string{"x.go"}}
	for _, l := range []Layer{CLI, Cmd, UI} {
		if got := checkProcess(p, l); len(got) != 0 {
			t.Errorf("%s flagged: %v", l, got)
		}
	}
	for _, l := range []Layer{Core, Lib, API, CAPI} {
		got := checkProcess(p, l)
		if len(got) != 1 || got[0].Rule != "process" || got[0].Subject != p.ImportPath+" uses fmt.Println" {
			t.Errorf("%s = %v", l, got)
		}
	}
}

func TestLoadPackagesSeesCgoFilesWithCgoDisabled(t *testing.T) {
	t.Setenv("CGO_ENABLED", "0")
	root := t.TempDir()
	files := map[string]string{
		"go.mod":        "module github.com/rommapp/grout\n\ngo 1.21\n",
		"capi/plain.go": "package capi\n",
		"capi/c.go":     "package capi\n\nimport \"C\"\nimport \"os\"\n\nfunc f() { os.Exit(1) }\n",
	}
	for name, body := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkgs, err := loadPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	var rules []string
	for _, v := range check(pkgs) {
		rules = append(rules, v.Key())
	}
	want := []string{"process github.com/rommapp/grout/capi uses os.Exit"}
	if !slices.Equal(rules, want) {
		t.Errorf("violations = %v, pkgs = %+v", rules, pkgs)
	}
}

func TestGlobalViolationNamesCmdAndCAPI(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\nvar n int\n\nfunc f() { n++ }\n"
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	vs := checkGlobals(pkg{ImportPath: "github.com/rommapp/grout/core/p", Dir: dir, GoFiles: []string{"a.go"}}, Core)
	if len(vs) != 1 {
		t.Fatalf("violations = %+v", vs)
	}
	if !strings.HasSuffix(vs[0].Detail, "only cmd and capi may hold process state") {
		t.Fatalf("detail = %q", vs[0].Detail)
	}
}

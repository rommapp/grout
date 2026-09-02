package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// violation is one broken rule, stable enough to list in allow.txt.
type violation struct {
	Rule    string   // layer, toolkit or global
	Subject string   // the canonical subject, e.g. "grout/ui -> grout/cache"
	Detail  string   // one line for a person
	Files   []string // where to look, relative to the repo root
}

// Key is the allow.txt line. Excludes line numbers and counts, which would
// churn on every edit.
func (v violation) Key() string { return v.Rule + " " + v.Subject }

func check(pkgs []pkg) []violation {
	var violations []violation
	for _, p := range pkgs {
		layer, ok := layerOf(p.ImportPath)
		if !ok {
			violations = append(violations, violation{
				Rule:    "layer",
				Subject: p.ImportPath + " -> (unclassified)",
				Detail: fmt.Sprintf("%s is not assigned to a layer; add it to layerRules in tools/archcheck/layers.go",
					p.ImportPath),
			})
			continue
		}

		violations = append(violations, checkImports(p, layer)...)
		violations = append(violations, checkGlobals(p, layer)...)
	}

	sort.Slice(violations, func(i, j int) bool { return violations[i].Key() < violations[j].Key() })
	return violations
}

// checkImports enforces the layer matrix and the toolkit rule.
func checkImports(p pkg, layer Layer) []violation {
	var violations []violation

	// Reported once per package, since the toolkit pulls in several
	// subpackages and the fix is the same. Must not stop the layer checks:
	// go list sorts imports, so github.com/... precedes every grout/... path.
	reportedToolkit := false

	for _, imported := range p.Imports {
		if strings.HasPrefix(imported, toolkitPrefix) {
			if !mayUseToolkit(layer) && !reportedToolkit {
				reportedToolkit = true
				violations = append(violations, violation{
					Rule:    "toolkit",
					Subject: p.ImportPath,
					Detail: fmt.Sprintf("%s (%s) imports the gabagool UI toolkit; only ui and cmd may",
						p.ImportPath, layer),
					Files: filesImporting(p, toolkitPrefix),
				})
			}
			continue
		}

		targetLayer, ok := layerOf(imported)
		if !ok || isExempt(imported) {
			continue // not a grout package, or a developer tool
		}

		if !allowedImports[layer][targetLayer] {
			violations = append(violations, violation{
				Rule:    "layer",
				Subject: p.ImportPath + " -> " + imported,
				Detail: fmt.Sprintf("%s (%s) must not import %s (%s)",
					p.ImportPath, layer, imported, targetLayer),
				Files: filesImporting(p, imported),
			})
		}
	}

	return violations
}

// checkGlobals reports package-level state that is actually mutated. Banning
// package-level vars outright would be mostly noise: embedded filesystems,
// error sentinels, regexes and lookup tables are written once.
func checkGlobals(p pkg, layer Layer) []violation {
	if mayHoldGlobalState(layer) {
		return nil
	}

	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(p.GoFiles))
	for _, name := range p.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(p.Dir, name), nil, 0)
		if err != nil {
			continue // a file we cannot parse is the compiler's problem, not ours
		}
		files = append(files, f)
	}

	declared := packageVars(files)
	if len(declared) == 0 {
		return nil
	}

	mutated := mutatedNames(files, declared)

	names := make([]string, 0, len(mutated))
	for name := range mutated {
		names = append(names, name)
	}
	sort.Strings(names)

	var violations []violation
	for _, name := range names {
		violations = append(violations, violation{
			Rule:    "global",
			Subject: p.ImportPath + "." + name,
			Detail: fmt.Sprintf("%s (%s) mutates package-level %s; only cmd may hold process state",
				p.ImportPath, layer, name),
		})
	}
	return violations
}

func packageVars(files []*ast.File) map[string]bool {
	declared := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range value.Names {
					if name.Name != "_" {
						declared[name.Name] = true
					}
				}
			}
		}
	}
	return declared
}

// mutableByMethod lists types changed by method call rather than assignment,
// which scanning for writes would miss.
var mutableByMethod = []string{"atomic.", "sync.Map"}

// mutatedNames returns declared names written after declaration, or whose type
// is mutated through its methods.
func mutatedNames(files []*ast.File, declared map[string]bool) map[string]bool {
	mutated := map[string]bool{}

	for _, f := range files {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || value.Type == nil {
					continue
				}
				typeName := exprString(value.Type)
				for _, prefix := range mutableByMethod {
					if strings.Contains(typeName, prefix) {
						for _, name := range value.Names {
							mutated[name.Name] = true
						}
					}
				}
			}
		}

		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					markIfPackageVar(lhs, declared, mutated)
				}
			case *ast.IncDecStmt:
				markIfPackageVar(node.X, declared, mutated)
			case *ast.UnaryExpr:
				// A pointer hands the value to someone who may write it.
				if node.Op == token.AND {
					markIfPackageVar(node.X, declared, mutated)
				}
			}
			return true
		})
	}

	return mutated
}

func markIfPackageVar(expr ast.Expr, declared, mutated map[string]bool) {
	if name := rootIdent(expr); name != "" && declared[name] {
		mutated[name] = true
	}
}

// rootIdent walks a selector or index chain to its base identifier, so writing
// to a field or key of a package-level var counts as mutating it.
func rootIdent(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return rootIdent(e.X)
	case *ast.IndexExpr:
		return rootIdent(e.X)
	case *ast.StarExpr:
		return rootIdent(e.X)
	case *ast.ParenExpr:
		return rootIdent(e.X)
	default:
		return ""
	}
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	default:
		return ""
	}
}

// filesImporting returns the files in p importing the given path, so a
// violation points somewhere to edit.
func filesImporting(p pkg, importPath string) []string {
	fset := token.NewFileSet()
	var found []string

	for _, name := range p.GoFiles {
		full := filepath.Join(p.Dir, name)
		f, err := parser.ParseFile(fset, full, nil, parser.ImportsOnly)
		if err != nil {
			continue
		}
		for _, spec := range f.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if path == importPath || strings.HasPrefix(path, importPath+"/") {
				if rel, err := filepath.Rel(repoRoot(), full); err == nil {
					found = append(found, rel)
				} else {
					found = append(found, full)
				}
				break
			}
		}
	}
	sort.Strings(found)
	return found
}

var cachedRoot string

func repoRoot() string {
	if cachedRoot == "" {
		cachedRoot, _ = filepath.Abs(".")
	}
	return cachedRoot
}

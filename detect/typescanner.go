package detect

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// TypeScanner uses go/packages for type-accurate analysis.
// It loads the full package graph so that range expressions are checked
// against their actual Go type, not a variable-name heuristic.
//
// When go/packages fails to load (missing dependencies, non-buildable state),
// TypeScanner falls back to the AST-only Scanner automatically.
type TypeScanner struct {
	rules RuleSet
}

// NewTypeScanner returns a TypeScanner with the given rule set.
func NewTypeScanner(rules RuleSet) *TypeScanner {
	return &TypeScanner{rules: rules}
}

// ScanDir loads all packages under root using go/packages and scans them.
// Falls back to the AST-only Scanner if packages cannot be loaded.
//
// excludePaths is a list of path prefixes (relative to root) to skip entirely,
// e.g. []string{"client/cli"}.
func (s *TypeScanner) ScanDir(root string, excludePaths ...string) (*ScanResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return NewScanner(s.rules).ScanDir(root)
	}

	cfg := &packages.Config{
		Mode: packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedTypes |
			packages.NeedFiles |
			packages.NeedImports, // required for cross-package type resolution
		Dir:   absRoot,
		Tests: false,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil || len(pkgs) == 0 {
		fmt.Fprintf(os.Stderr, "warn: go/packages load failed for %s, falling back to AST-only analysis\n", root)
		return NewScanner(s.rules).ScanDir(root)
	}

	idx := s.rules.index()
	result := &ScanResult{}

	for _, pkg := range pkgs {
		typesInfo := pkg.TypesInfo
		if len(pkg.Errors) > 0 {
			for _, e := range pkg.Errors {
				fmt.Fprintf(os.Stderr, "warn: go/packages: %s: %v, using AST-only for this package\n", pkg.PkgPath, e)
			}
			typesInfo = nil
		}

		if len(pkg.Syntax) > 0 {
			for _, f := range pkg.Syntax {
				filename := pkg.Fset.File(f.Pos()).Name()
				if shouldSkipFile(filename) {
					continue
				}
				rel, err := filepath.Rel(absRoot, filename)
				if err != nil {
					rel = filename
				}
				if shouldSkipRelPath(rel, excludePaths) {
					continue
				}
				result.Files++
				result.Findings = append(result.Findings,
					scanFileWithTypes(pkg.Fset, f, rel, idx, typesInfo)...)
			}
		} else if len(pkg.Errors) > 0 {
			// pkg.Syntax unavailable; re-parse source files for AST-only analysis.
			for _, filename := range pkg.GoFiles {
				if shouldSkipFile(filename) {
					continue
				}
				rel, err := filepath.Rel(absRoot, filename)
				if err != nil {
					rel = filename
				}
				if shouldSkipRelPath(rel, excludePaths) {
					continue
				}
				fset := token.NewFileSet()
				f, parseErr := parser.ParseFile(fset, filename, nil, 0)
				if parseErr != nil {
					continue
				}
				result.Files++
				result.Findings = append(result.Findings,
					scanFileWithTypes(fset, f, rel, idx, nil)...)
			}
		}
	}
	return result, nil
}

func shouldSkipFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	// Skip vendor and testutil directories.
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "vendor" || part == "testutil" || part == "mock" {
			return true
		}
	}
	return false
}

// shouldSkipRelPath returns true when the relative file path matches any of
// the provided exclude prefixes (e.g. "client/cli").
func shouldSkipRelPath(rel string, excludePaths []string) bool {
	rel = filepath.ToSlash(rel)
	for _, prefix := range excludePaths {
		prefix = filepath.ToSlash(prefix)
		if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
			return true
		}
	}
	return false
}

// abciHookFunctions are block-level ABCI callbacks. Time and I/O calls inside
// these functions are not in the tx execution path and do not cause BlockSTM
// violations. They are excluded from all findings.
var abciHookFunctions = map[string]bool{
	"BeginBlocker": true,
	"EndBlocker":   true,
	"PreBlocker":   true,
	"PreBlock":     true,
	"EndBlock":     true,
	"BeginBlock":   true,
}


// scanFileWithTypes is like Scanner.ScanFile but uses typesInfo for
// type-accurate RangeStmt detection.
func scanFileWithTypes(
	fset *token.FileSet,
	f *ast.File,
	relPath string,
	idx *ruleIndex,
	typesInfo *types.Info,
) []Finding {
	aliases := buildAliasMap(f)
	module := ModuleFromPath(relPath)

	var findings []Finding
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if abciHookFunctions[fn.Name.Name] {
			continue
		}
		findings = append(findings, scanBodyWithTypes(fset, fn.Body, fn.Name.Name, relPath, module, idx, aliases, typesInfo)...)
	}
	return findings
}

func scanBodyWithTypes(
	fset *token.FileSet,
	body ast.Node,
	funcName, relPath, module string,
	idx *ruleIndex,
	aliases map[string]string,
	typesInfo *types.Info,
) []Finding {
	var findings []Finding

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			findings = append(findings, scanBodyWithTypes(fset, node.Body, funcName+".closure", relPath, module, idx, aliases, typesInfo)...)
			return false

		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			importPath, ok := aliases[ident.Name]
			if !ok {
				return true
			}
			cat, ok := idx.Lookup(importPath, sel.Sel.Name)
			if !ok {
				return true
			}
			pos := fset.Position(node.Pos())
			findings = append(findings, Finding{
				Category: cat,
				File:     relPath,
				Line:     pos.Line,
				FuncName: funcName,
				Call:     importPath + "." + sel.Sel.Name,
				Module:   module,
			})

		case *ast.RangeStmt:
			// Type-accurate map detection: check the concrete type of the
			// ranged expression. Falls back to the name heuristic when type
			// information is unavailable (e.g. the expression is unresolved).
			if isMapRange(node.X, typesInfo) {
				pos := fset.Position(node.Pos())
				name := exprName(node.X)
				findings = append(findings, Finding{
					Category: CatMapIter,
					File:     relPath,
					Line:     pos.Line,
					FuncName: funcName,
					Call:     "range " + name,
					Module:   module,
				})
			}
		}
		return true
	})
	return findings
}

// isMapRange returns true when expr has an underlying map type.
// When typesInfo is nil or the type is unknown, falls back to the name heuristic.
func isMapRange(expr ast.Expr, typesInfo *types.Info) bool {
	if typesInfo != nil {
		if t := typesInfo.TypeOf(expr); t != nil {
			_, isMap := t.Underlying().(*types.Map)
			return isMap
		}
	}
	// Fallback: name heuristic for unresolved expressions.
	return mapLikeName(expr) != ""
}

// exprName returns a short readable name for an expression (for Finding.Call).
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	default:
		return "map"
	}
}

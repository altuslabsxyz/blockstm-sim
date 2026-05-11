package lint

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

// kvStorePkgPrefixes lists package path prefixes whose types are KVStore-backed
// and therefore safe from out-of-KVStore mutation findings.
// This is the type-accurate replacement for the name-based isSafeField().
var kvStorePkgPrefixes = []string{
	"cosmossdk.io/core/store",
	"cosmossdk.io/collections",
	"cosmossdk.io/store",
	"github.com/cosmos/cosmos-sdk/store",
	"github.com/cosmos/cosmos-sdk/types",
	"github.com/cosmos/cosmos-sdk/codec",
	"github.com/cosmos/cosmos-sdk/baseapp",
	"github.com/cosmos/cosmos-sdk/types/module",
	"github.com/cometbft/cometbft",
	"google.golang.org/grpc",
	"cosmossdk.io/log",
}

func isKVStoreBacked(pkg string) bool {
	for _, prefix := range kvStorePkgPrefixes {
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}

// ScanDirWithTypes loads packages via go/packages for type-accurate keeper
// field analysis. Falls back to the AST-only ScanDir when packages cannot be
// loaded (missing dependencies, non-buildable state).
func (s *Scanner) ScanDirWithTypes(root string) (*LintResult, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return s.ScanDir(root)
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
		return s.ScanDir(root)
	}

	result := &LintResult{}
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
				if shouldSkipLintFile(filename) {
					continue
				}
				rel, err := filepath.Rel(absRoot, filename)
				if err != nil {
					rel = filename
				}
				result.Files++
				result.Findings = append(result.Findings,
					s.scanFileWithTypes(pkg.Fset, f, rel, typesInfo)...)
			}
		} else if len(pkg.Errors) > 0 {
			// pkg.Syntax unavailable; re-parse source files for AST-only analysis.
			for _, filename := range pkg.GoFiles {
				if shouldSkipLintFile(filename) {
					continue
				}
				rel, err := filepath.Rel(absRoot, filename)
				if err != nil {
					rel = filename
				}
				fset := token.NewFileSet()
				f, parseErr := parser.ParseFile(fset, filename, nil, 0)
				if parseErr != nil {
					continue
				}
				result.Files++
				result.Findings = append(result.Findings, s.ScanFile(fset, f, rel)...)
			}
		}
	}
	return result, nil
}

func shouldSkipLintFile(path string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return true
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "vendor" || part == "testutil" || part == "mock" {
			return true
		}
	}
	return false
}


// scanFileWithTypes is like ScanFile but uses typesInfo for type-accurate
// keeper field classification.
func (s *Scanner) scanFileWithTypes(
	fset *token.FileSet,
	f *ast.File,
	relPath string,
	typesInfo *types.Info,
) []Finding {
	module := ModuleFromPath(relPath)
	pkgVars := collectPkgVars(f)

	var findings []Finding

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		receiverName, hasReceiver := receiverIdent(fn)

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					pos := fset.Position(lhs.Pos())

					if hasReceiver && !isConstructor(fn.Name.Name) {
						if field := receiverField(lhs, receiverName); field != "" {
							// Extract the SelectorExpr (k.field) from the LHS —
							// unwrapping IndexExpr chains like k.cache[key] — so
							// isTypeSafeField sees the field's declared type, not
							// the map element type.
							sel := fieldSelectorFromLHS(lhs)
							if !isTypeSafeField(sel, typesInfo) && !isSafeField(field) {
								findings = append(findings, Finding{
									Kind:   KindKeeperField,
									File:   relPath,
									Line:   pos.Line,
									Method: fn.Name.Name,
									Target: field,
									Module: module,
								})
							}
						}
					}

					if !isConstructor(fn.Name.Name) {
						if v := pkgVarTarget(lhs, pkgVars); v != "" {
							findings = append(findings, Finding{
								Kind:   KindPkgVar,
								File:   relPath,
								Line:   pos.Line,
								Method: fn.Name.Name,
								Target: v,
								Module: module,
							})
						}
					}
				}

			case *ast.IncDecStmt:
				pos := fset.Position(node.Pos())

				if hasReceiver && !isConstructor(fn.Name.Name) {
					if field := receiverField(node.X, receiverName); field != "" {
						sel := fieldSelectorFromLHS(node.X)
						if !isTypeSafeField(sel, typesInfo) && !isSafeField(field) {
							findings = append(findings, Finding{
								Kind:   KindKeeperField,
								File:   relPath,
								Line:   pos.Line,
								Method: fn.Name.Name,
								Target: field,
								Module: module,
							})
						}
					}
				}

				if !isConstructor(fn.Name.Name) {
					if v := pkgVarTarget(node.X, pkgVars); v != "" {
						findings = append(findings, Finding{
							Kind:   KindPkgVar,
							File:   relPath,
							Line:   pos.Line,
							Method: fn.Name.Name,
							Target: v,
							Module: module,
						})
					}
				}
			}
			return true
		})
	}
	return findings
}

// fieldSelectorFromLHS walks through IndexExpr chains to find the underlying
// SelectorExpr (e.g., k.cache from k.cache[key]). Returns the original expr
// when no SelectorExpr is found.
func fieldSelectorFromLHS(expr ast.Expr) ast.Expr {
	for {
		switch e := expr.(type) {
		case *ast.IndexExpr:
			expr = e.X
		case *ast.SelectorExpr:
			return expr
		default:
			return expr
		}
	}
}

// isTypeSafeField returns true when the expression's type comes from a
// KVStore-backed package, meaning the field is safe under BlockSTM.
// expr should be the SelectorExpr for the field (k.field), not a raw LHS —
// use fieldSelectorFromLHS to extract it from compound expressions.
// When typesInfo is nil or the type is unresolved, returns false so the
// name-based isSafeField() fallback applies.
func isTypeSafeField(expr ast.Expr, typesInfo *types.Info) bool {
	if typesInfo == nil {
		return false
	}
	t := typesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	// Dereference pointer types.
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return false
	}
	return isKVStoreBacked(pkg.Path())
}

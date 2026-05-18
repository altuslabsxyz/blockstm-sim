package lint

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Scanner walks Go source files and reports keeper-safety lint findings.
type Scanner struct{}

// NewScanner returns a Scanner with default configuration.
func NewScanner() *Scanner { return &Scanner{} }

// shouldSkipRelPath returns true when rel matches any of the given path
// prefixes (relative to the scan root), e.g. "x/bank" or "client/cli".
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

// ScanDir recursively scans all non-test .go files under root.
// Vendor and testutil directories are skipped. excludePaths is a list of
// path prefixes (relative to root) to skip entirely, e.g. "x/bank".
func (s *Scanner) ScanDir(root string, excludePaths ...string) (*LintResult, error) {
	result := &LintResult{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testutil" || name == "mock" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if shouldSkipRelPath(rel, excludePaths) {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil // skip unparseable files
		}
		result.Files++
		result.Findings = append(result.Findings, s.ScanFile(fset, f, rel)...)
		return nil
	})
	return result, err
}

// ScanFile inspects a single parsed file for keeper-safety violations.
func (s *Scanner) ScanFile(fset *token.FileSet, f *ast.File, relPath string) []Finding {
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

								// keeper field: k.field = ...  or  k.field[x] = ...
					// Checked in all methods (not just ctx-bearing) because helper
					// functions called from handlers also mutate out-of-KV state.
					// Constructor-like functions (New*, Init*) are excluded.
					if hasReceiver && !isConstructor(fn.Name.Name) {
						if field := receiverField(lhs, receiverName); field != "" && !isSafeField(field) {
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

					// package-level variable: pkgVar = ...
					// Checked in all non-constructor methods for the same reason as
					// keeper_field: helper functions without ctx can be called from
					// context-bearing handlers and still cause BlockSTM races.
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

				// k.field++  /  k.field--
				if hasReceiver && !isConstructor(fn.Name.Name) {
					if field := receiverField(node.X, receiverName); field != "" && !isSafeField(field) {
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

				// pkgVar++
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

// receiverIdent returns the name of the method receiver and true if the
// function has exactly one receiver. Returns ("", false) for plain functions.
func receiverIdent(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false
	}
	field := fn.Recv.List[0]
	if len(field.Names) == 0 {
		return "", false
	}
	return field.Names[0].Name, true
}


// receiverField returns the field name if expr is `receiverName.field` or
// `receiverName.field[...]`. Returns "" otherwise.
func receiverField(expr ast.Expr, receiverName string) string {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := e.X.(*ast.Ident); ok && ident.Name == receiverName {
			return e.Sel.Name
		}
	case *ast.IndexExpr:
		return receiverField(e.X, receiverName)
	case *ast.IndexListExpr: // generics
		return receiverField(e.X, receiverName)
	}
	return ""
}

// isConstructor returns true for function names that are constructor or init
// patterns where field assignments are expected and safe.
// isConstructor returns true for function names that are constructor, init, or
// generated (protobuf Marshal/Unmarshal) patterns where field assignments are
// expected and not related to transaction-time execution.
func isConstructor(name string) bool {
	return strings.HasPrefix(name, "New") ||
		strings.HasPrefix(name, "Init") ||
		strings.HasPrefix(name, "Setup") ||
		strings.HasPrefix(name, "Unmarshal") ||
		strings.HasPrefix(name, "Marshal") ||
		name == "Reset" ||
		name == "ProtoMessage" ||
		name == "init"
}

// isSafeField returns true for field names that are conventionally immutable
// references (store keys, codecs, other keepers) rather than mutable state.
// This reduces false positives for common keeper patterns.
var safeFieldSuffixes = []string{
	"Key", "key",
	"cdc", "Cdc", "Codec", "codec",
	"Keeper", "keeper",
	"Router", "router",
	"Config", "config",
	"authority", "Authority",
	"Logger", "logger",
}

func isSafeField(name string) bool {
	for _, suffix := range safeFieldSuffixes {
		if strings.HasSuffix(name, suffix) || name == suffix {
			return true
		}
	}
	return false
}

// collectPkgVars returns the set of package-level variable names declared in f.
func collectPkgVars(f *ast.File) map[string]bool {
	vars := make(map[string]bool)
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name.Name != "_" {
					vars[name.Name] = true
				}
			}
		}
	}
	return vars
}

// pkgVarTarget returns the variable name if expr is a direct reference to one
// of the package-level variables in the given set. Returns "" otherwise.
func pkgVarTarget(expr ast.Expr, pkgVars map[string]bool) string {
	if ident, ok := expr.(*ast.Ident); ok && pkgVars[ident.Name] {
		return ident.Name
	}
	return ""
}

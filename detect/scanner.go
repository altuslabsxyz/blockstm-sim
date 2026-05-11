package detect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Scanner struct {
	idx *ruleIndex
}

func NewScanner(rules RuleSet) *Scanner {
	return &Scanner{idx: rules.index()}
}

func (s *Scanner) ScanDir(root string) (*ScanResult, error) {
	result := &ScanResult{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || name == "testutil" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		result.Files++
		result.Findings = append(result.Findings, s.ScanFile(fset, f, rel)...)
		return nil
	})
	return result, err
}

func (s *Scanner) ScanFile(fset *token.FileSet, f *ast.File, relPath string) []Finding {
	aliases := buildAliasMap(f)
	module := ModuleFromPath(relPath)

	var findings []Finding
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		findings = append(findings, s.scanBody(fset, fn.Body, fn.Name.Name, relPath, module, aliases)...)
	}
	return findings
}

// scanBody walks a function body and reports findings, recursing into closures
// with their own name so that FuncName in each Finding accurately reflects the
// enclosing function or closure rather than the last seen FuncDecl.
func (s *Scanner) scanBody(fset *token.FileSet, body ast.Node, funcName, relPath, module string, aliases map[string]string) []Finding {
	var findings []Finding

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			// Process each closure separately so findings inside it report
			// "outerFunc.closure" rather than the outer function's name.
			findings = append(findings, s.scanBody(fset, node.Body, funcName+".closure", relPath, module, aliases)...)
			return false // Inspect must not recurse into this subtree again

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
			cat, ok := s.idx.Lookup(importPath, sel.Sel.Name)
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
			if name := mapLikeName(node.X); name != "" {
				pos := fset.Position(node.Pos())
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

// mapLikeName returns the name of the expression being ranged over when it
// heuristically looks like a Go map. Returns "" for slices, channels, and
// other non-map expressions.
//
// Heuristic: the variable or field name contains a map-suggestive substring.
// This produces false positives (e.g. a slice named "indexMap") but the PRD
// explicitly accepts heuristic findings for this category.
func mapLikeName(expr ast.Expr) string {
	var name string
	switch e := expr.(type) {
	case *ast.Ident:
		name = e.Name
	case *ast.SelectorExpr:
		name = e.Sel.Name
	default:
		return ""
	}
	lower := strings.ToLower(name)
	for _, kw := range []string{"map", "cache", "index", "registry", "table", "store", "dict", "lookup"} {
		if strings.Contains(lower, kw) {
			return name
		}
	}
	return ""
}

func buildAliasMap(f *ast.File) map[string]string {
	m := make(map[string]string)
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil {
			alias = imp.Name.Name
			if alias == "_" || alias == "." {
				continue
			}
		} else {
			parts := strings.Split(path, "/")
			alias = parts[len(parts)-1]
		}
		m[alias] = path
	}
	return m
}

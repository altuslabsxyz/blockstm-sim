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
	var enclosingFunc string

	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			enclosingFunc = node.Name.Name
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
				FuncName: enclosingFunc,
				Call:     importPath + "." + sel.Sel.Name,
				Module:   module,
			})
		}
		return true
	})

	return findings
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

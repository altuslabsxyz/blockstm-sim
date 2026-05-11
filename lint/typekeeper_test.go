package lint

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFieldSelectorFromLHS_IndexExpr(t *testing.T) {
	sel := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "k"},
		Sel: &ast.Ident{Name: "cache"},
	}
	got := fieldSelectorFromLHS(&ast.IndexExpr{X: sel})
	require.Equal(t, sel, got, "single-index expr should be unwrapped to SelectorExpr")
}

func TestFieldSelectorFromLHS_IndexListExpr(t *testing.T) {
	// k.cache[string, int64] — generic two-param indexing
	sel := &ast.SelectorExpr{
		X:   &ast.Ident{Name: "k"},
		Sel: &ast.Ident{Name: "cache"},
	}
	got := fieldSelectorFromLHS(&ast.IndexListExpr{
		X:       sel,
		Indices: []ast.Expr{&ast.Ident{Name: "string"}, &ast.Ident{Name: "int64"}},
	})
	_, ok := got.(*ast.SelectorExpr)
	require.True(t, ok, "IndexListExpr should be unwrapped to SelectorExpr, not returned as-is")
	require.Equal(t, sel, got)
}

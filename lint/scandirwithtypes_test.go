package lint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/lint"
)

// TestScanDirWithTypes_PartialLoadError verifies that a single package load
// error does not cause a total fallback to AST-only analysis for all packages.
// Findings from packages that loaded successfully must be preserved.
func TestScanDirWithTypes_PartialLoadError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testmod\n\ngo 1.21\n"), 0o644))

	// good package: loads fine and has a keeper-field violation on k.cache
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "x", "bank", "keeper"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x", "bank", "keeper", "keeper.go"), []byte(`package keeper

type Keeper struct{ cache map[string]int64 }

func (k *Keeper) Set(val int64) { k.cache["key"] = val }
`), 0o644))

	// bad package: parses fine but fails type-checking
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "x", "broken"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x", "broken", "broken.go"), []byte(`package broken

var x int = "string causes a type error"
`), 0o644))

	s := lint.NewScanner()
	result, err := s.ScanDirWithTypes(dir)
	require.NoError(t, err)

	var keeperFindings []lint.Finding
	for _, f := range result.Findings {
		if f.Kind == lint.KindKeeperField {
			keeperFindings = append(keeperFindings, f)
		}
	}
	require.Len(t, keeperFindings, 1, "good package findings must not be lost due to bad package load error")
	require.Equal(t, "cache", keeperFindings[0].Target)
	require.Equal(t, "Set", keeperFindings[0].Method)
}

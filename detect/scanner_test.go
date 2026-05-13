package detect

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Map iteration heuristic
// ---------------------------------------------------------------------------

func TestScanFile_MapIter_Detected(t *testing.T) {
	src := `package keeper
type Keeper struct{ cache map[string]int64 }
func (k *Keeper) Process(ctx sdk.Context) {
	for key, val := range k.cache {
		_ = key
		_ = val
	}
}`
	findings := scanSource(t, src, "x/bank/keeper/keeper.go")
	var mapFindings []Finding
	for _, f := range findings {
		if f.Category == CatMapIter {
			mapFindings = append(mapFindings, f)
		}
	}
	require.Len(t, mapFindings, 1)
	require.Equal(t, "range cache", mapFindings[0].Call)
	require.Equal(t, "Process", mapFindings[0].FuncName)
}

func TestScanFile_MapIter_SliceNotFlagged(t *testing.T) {
	src := `package keeper
func Process(items []string) {
	for _, item := range items {
		_ = item
	}
}`
	findings := scanSource(t, src, "x/bank/keeper/keeper.go")
	for _, f := range findings {
		require.NotEqual(t, CatMapIter, f.Category, "slice range must not be flagged")
	}
}

func TestScanFile_MapIter_RegistryFlagged(t *testing.T) {
	src := `package keeper
var moduleRegistry map[string]bool
func init() {
	for mod := range moduleRegistry {
		_ = mod
	}
}`
	findings := scanSource(t, src, "x/bank/keeper/keeper.go")
	var mapFindings []Finding
	for _, f := range findings {
		if f.Category == CatMapIter {
			mapFindings = append(mapFindings, f)
		}
	}
	require.Len(t, mapFindings, 1)
	require.Equal(t, "range moduleRegistry", mapFindings[0].Call)
}

// ---------------------------------------------------------------------------
// Pointer address heuristic
// ---------------------------------------------------------------------------

func TestScanFile_Pointer_ReflectValueOf(t *testing.T) {
	src := `package keeper
import "reflect"
func leakAddr(x interface{}) uintptr {
	return reflect.ValueOf(x).Pointer()
}`
	findings := scanSource(t, src, "x/bank/keeper/keeper.go")
	var ptrFindings []Finding
	for _, f := range findings {
		if f.Category == CatPointer {
			ptrFindings = append(ptrFindings, f)
		}
	}
	require.Len(t, ptrFindings, 1)
	require.Equal(t, "reflect.ValueOf", ptrFindings[0].Call)
}

func TestScanFile_TimeNow(t *testing.T) {
	src := `package foo

import "time"

func DoWork() {
	_ = time.Now()
}
`
	findings := scanSource(t, src, "x/bank/keeper/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, CatTime, findings[0].Category)
	require.Equal(t, "time.Now", findings[0].Call)
	require.Equal(t, "DoWork", findings[0].FuncName)
	require.Equal(t, "bank", findings[0].Module)
	require.Equal(t, 6, findings[0].Line)
}

func TestScanFile_ImportAlias(t *testing.T) {
	src := `package foo

import t "time"

func Tick() {
	_ = t.Now()
}
`
	findings := scanSource(t, src, "x/staking/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, "time.Now", findings[0].Call)
	require.Equal(t, "staking", findings[0].Module)
}

func TestScanFile_CryptoRand(t *testing.T) {
	src := `package foo

import "crypto/rand"

func GenKey() {
	buf := make([]byte, 32)
	rand.Read(buf)
}
`
	findings := scanSource(t, src, "x/auth/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, CatRand, findings[0].Category)
	require.Equal(t, "crypto/rand.Read", findings[0].Call)
}

func TestScanFile_MathRandPackageLevel(t *testing.T) {
	src := `package foo

import "math/rand"

func Pick() int {
	return rand.Intn(100)
}
`
	findings := scanSource(t, src, "x/bank/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, CatRand, findings[0].Category)
	require.Equal(t, "math/rand.Intn", findings[0].Call)
}

func TestScanFile_MathRandMethodCall_NoFlag(t *testing.T) {
	src := `package foo

import "math/rand"

func Pick() int {
	r := rand.New(rand.NewSource(42))
	return r.Intn(100)
}
`
	findings := scanSource(t, src, "x/bank/foo.go")
	require.Empty(t, findings)
}

func TestScanFile_OsGetenv(t *testing.T) {
	src := `package foo

import "os"

func ReadEnv() string {
	return os.Getenv("HOME")
}
`
	findings := scanSource(t, src, "baseapp/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, CatIO, findings[0].Category)
	require.Equal(t, "os.Getenv", findings[0].Call)
	require.Equal(t, "baseapp", findings[0].Module)
}

func TestScanFile_NetHTTP(t *testing.T) {
	src := `package foo

import "net/http"

func Fetch() {
	http.Get("http://example.com")
}
`
	findings := scanSource(t, src, "x/oracle/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, CatIO, findings[0].Category)
	require.Equal(t, "net/http.Get", findings[0].Call)
}

func TestScanFile_MultipleFindings(t *testing.T) {
	src := `package foo

import (
	"os"
	"time"
)

func A() {
	_ = time.Now()
}

func B() {
	os.Getenv("X")
}
`
	findings := scanSource(t, src, "x/bank/foo.go")
	require.Len(t, findings, 2)

	sort.Slice(findings, func(i, j int) bool { return findings[i].Line < findings[j].Line })
	require.Equal(t, CatTime, findings[0].Category)
	require.Equal(t, CatIO, findings[1].Category)
}

func TestScanFile_NoForbiddenCalls(t *testing.T) {
	src := `package foo

import "time"

func Elapsed(d time.Duration) bool {
	return d > 0
}
`
	findings := scanSource(t, src, "x/bank/foo.go")
	require.Empty(t, findings)
}

func TestScanFile_TopLevelInit(t *testing.T) {
	src := `package foo

import "time"

func init() {
	_ = time.Now()
}
`
	findings := scanSource(t, src, "x/bank/foo.go")
	require.Len(t, findings, 1)
	require.Equal(t, "init", findings[0].FuncName)
}

func TestScanDir_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x", "bank", "foo.go"), `package bank
import "time"
func F() { _ = time.Now() }
`)
	writeFile(t, filepath.Join(dir, "x", "bank", "foo_test.go"), `package bank
import "time"
func TestF() { _ = time.Now() }
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	require.Equal(t, "x/bank/foo.go", result.Findings[0].File)
	require.Equal(t, 1, result.Files)
}

func TestScanDir_SkipsVendor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "vendor", "lib", "foo.go"), `package lib
import "time"
func F() { _ = time.Now() }
`)
	writeFile(t, filepath.Join(dir, "x", "bank", "ok.go"), `package bank
func G() {}
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.Empty(t, result.Findings)
	require.Equal(t, 1, result.Files)
}

func TestScanDir_SkipsTestutil(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "testutil", "foo.go"), `package testutil
import "time"
func F() { _ = time.Now() }
`)
	writeFile(t, filepath.Join(dir, "x", "bank", "ok.go"), `package bank
func G() {}
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.Empty(t, result.Findings)
	require.Equal(t, 1, result.Files)
}

// ---------------------------------------------------------------------------
// ABCI hook function filter
// ---------------------------------------------------------------------------

func TestScanFile_BeginBlocker_TimeNow_NotFlagged(t *testing.T) {
	src := `package keeper
import "time"
func BeginBlocker(ctx sdk.Context) {
	_ = time.Now()
}`
	findings := scanSource(t, src, "x/budget/keeper/abci.go")
	require.Empty(t, findings, "time.Now inside BeginBlocker must not be flagged")
}

func TestScanFile_EndBlocker_NotFlagged(t *testing.T) {
	src := `package keeper
import "time"
func EndBlocker(ctx sdk.Context) {
	_ = time.Now()
}`
	findings := scanSource(t, src, "x/staking/keeper/abci.go")
	require.Empty(t, findings, "time.Now inside EndBlocker must not be flagged")
}

func TestScanFile_PreBlocker_NotFlagged(t *testing.T) {
	src := `package keeper
import "time"
func PreBlocker(ctx sdk.Context) {
	_ = time.Now()
}`
	findings := scanSource(t, src, "x/upgrade/keeper/abci.go")
	require.Empty(t, findings)
}

func TestScanFile_NonHookFunction_StillFlagged(t *testing.T) {
	// A function called BeginWork (not BeginBlocker) must still be scanned.
	src := `package keeper
import "time"
func BeginWork(ctx sdk.Context) {
	_ = time.Now()
}`
	findings := scanSource(t, src, "x/bank/keeper/work.go")
	require.Len(t, findings, 1, "non-ABCI hook function must still be scanned")
	require.Equal(t, "BeginWork", findings[0].FuncName)
}

// ---------------------------------------------------------------------------
// Path exclusion (ScanDir level)
// ---------------------------------------------------------------------------

func TestScanDir_ExcludePath_SkipsMatchingPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "client", "cli", "utils.go"), `package cli
import "os"
func ParseMetadata() { os.ReadFile("f") }
`)
	writeFile(t, filepath.Join(dir, "x", "bank", "keeper.go"), `package bank
func Keep() {}
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir, "client/cli")
	require.NoError(t, err)
	require.Empty(t, result.Findings, "client/cli path must be excluded")
}

func TestScanDir_ExcludePath_NonMatchingStillScanned(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "client", "cli", "utils.go"), `package cli
import "os"
func ParseMetadata() { os.ReadFile("f") }
`)
	writeFile(t, filepath.Join(dir, "x", "bank", "keeper.go"), `package bank
import "os"
func Keep() { os.Getenv("X") }
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir, "client/cli")
	require.NoError(t, err)
	require.Len(t, result.Findings, 1)
	require.Equal(t, "x/bank/keeper.go", result.Findings[0].File)
}

func TestScanDir_ExcludePath_MultipleExclusions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "client", "cli", "a.go"), `package cli
import "os"
func A() { os.ReadFile("f") }
`)
	writeFile(t, filepath.Join(dir, "scripts", "b.go"), `package scripts
import "time"
func B() { _ = time.Now() }
`)
	writeFile(t, filepath.Join(dir, "x", "bank", "c.go"), `package bank
func C() {}
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir, "client/cli", "scripts")
	require.NoError(t, err)
	require.Empty(t, result.Findings)
}

func scanSource(t *testing.T, src, filename string) []Finding {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	require.NoError(t, err)

	s := NewScanner(DefaultRules())
	return s.ScanFile(fset, f, filename)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

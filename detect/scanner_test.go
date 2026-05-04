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

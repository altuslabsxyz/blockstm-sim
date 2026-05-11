package detect

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegration_ScanDir_MixedFindings(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "x", "bank", "keeper", "send.go"), `package keeper

import "time"

func SendCoins() {
	_ = time.Now()
}
`)

	writeFile(t, filepath.Join(dir, "x", "staking", "keeper", "validator.go"), `package keeper

import "math/rand"

func SelectValidator() int {
	return rand.Intn(100)
}
`)

	writeFile(t, filepath.Join(dir, "baseapp", "state.go"), `package baseapp

import "os"

func LoadConfig() string {
	return os.Getenv("HOME")
}
`)

	writeFile(t, filepath.Join(dir, "x", "auth", "keeper", "clean.go"), `package keeper

func Clean() string {
	return "no forbidden calls"
}
`)

	writeFile(t, filepath.Join(dir, "x", "bank", "keeper", "send_test.go"), `package keeper

import "time"

func TestSend() {
	_ = time.Now()
}
`)

	writeFile(t, filepath.Join(dir, "vendor", "lib", "lib.go"), `package lib

import "time"

func F() { _ = time.Now() }
`)

	writeFile(t, filepath.Join(dir, "testutil", "helper.go"), `package testutil

import "os"

func H() { os.Getenv("X") }
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir)
	require.NoError(t, err)

	require.Equal(t, 4, result.Files)
	require.Len(t, result.Findings, 3)

	byCall := map[string]Finding{}
	for _, f := range result.Findings {
		byCall[f.Call] = f
	}

	tf := byCall["time.Now"]
	require.Equal(t, CatTime, tf.Category)
	require.Equal(t, "bank", tf.Module)
	require.Equal(t, "SendCoins", tf.FuncName)

	rf := byCall["math/rand.Intn"]
	require.Equal(t, CatRand, rf.Category)
	require.Equal(t, "staking", rf.Module)
	require.Equal(t, "SelectValidator", rf.FuncName)

	iof := byCall["os.Getenv"]
	require.Equal(t, CatIO, iof.Category)
	require.Equal(t, "baseapp", iof.Module)
	require.Equal(t, "LoadConfig", iof.FuncName)
}

func TestIntegration_CategoryFilter(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "x", "bank", "foo.go"), `package bank

import (
	"os"
	"time"
)

func A() { _ = time.Now() }
func B() { os.Getenv("X") }
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir)
	require.NoError(t, err)
	require.Len(t, result.Findings, 2)

	var timeOnly []Finding
	for _, f := range result.Findings {
		if f.Category == CatTime {
			timeOnly = append(timeOnly, f)
		}
	}
	require.Len(t, timeOnly, 1)
	require.Equal(t, "time.Now", timeOnly[0].Call)
}

func TestIntegration_ReportOutput(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "x", "bank", "foo.go"), `package bank

import "time"

func DoWork() {
	_ = time.Now()
}
`)

	s := NewScanner(DefaultRules())
	result, err := s.ScanDir(dir)
	require.NoError(t, err)

	var buf bytes.Buffer
	rep := NewReporter(&buf)
	rep.Header(dir)
	for _, f := range result.Findings {
		rep.Finding(f)
	}
	rep.Footer(result, dir)

	out := buf.String()
	require.Contains(t, out, "Detect  sdk-path=")
	require.Contains(t, out, "[time]")
	require.Contains(t, out, "time.Now")
	require.Contains(t, out, "1 findings / 1 time / 0 rand / 0 io")
}

// TestTypeScannerScanDir_PartialLoadError verifies that a single package load
// error does not cause a total fallback to AST-only analysis for all packages.
//
// The "good" package ranges over a []string named indexMap. The name heuristic
// (AST-only) fires on "indexMap" because it contains "map". The type-accurate
// path correctly identifies it as a slice and produces no finding. If the bug
// regresses (total fallback), CatMapIter appears and the assertion fails.
func TestTypeScannerScanDir_PartialLoadError(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module testmod\n\ngo 1.21\n"), 0o644))

	// good package: type-accurate sees []string, suppresses the false-positive finding
	writeFile(t, filepath.Join(dir, "good", "good.go"), `package good

func F(indexMap []string) {
	for _, v := range indexMap {
		_ = v
	}
}
`)

	// bad package: parses fine but fails type-checking
	writeFile(t, filepath.Join(dir, "bad", "bad.go"), `package bad

var x int = "string causes a type error"
`)

	ts := NewTypeScanner(DefaultRules())
	result, err := ts.ScanDir(dir)
	require.NoError(t, err)

	for _, f := range result.Findings {
		require.NotEqual(t, CatMapIter, f.Category,
			"indexMap is a []string; type-accurate scanner must not flag it (got finding: %+v)", f)
	}
}

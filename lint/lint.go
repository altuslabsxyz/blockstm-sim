// Package lint provides static analysis for BlockSTM safety violations in keeper code.
// It detects assignments to keeper struct fields that bypass the KVStore wrapper
// and writes to package-level variables inside context-bearing methods.
package lint

import "strings"

// Kind classifies the type of lint finding.
type Kind string

const (
	// KindKeeperField flags a direct assignment to a receiver struct field
	// inside a method, bypassing the KVStore wrapper. Under BlockSTM parallel
	// execution, concurrent reads and writes to such fields produce stale-read
	// violations that diverge AppHash.
	KindKeeperField Kind = "keeper_field"

	// KindPkgVar flags a write to a package-level variable inside a
	// context-bearing method. Package-level state is shared across all
	// goroutines and invisible to BlockSTM's MVMemory tracker.
	KindPkgVar Kind = "pkg_var"
)

// Finding describes a single lint violation.
type Finding struct {
	Kind   Kind
	File   string
	Line   int
	Method string // enclosing function name
	Target string // field name or variable name being written
	Module string // module inferred from path (e.g. "bank")
}

// LintResult holds the aggregate output of a lint scan.
type LintResult struct {
	Findings []Finding
	Files    int
}

// ModuleFromPath infers a Cosmos module name from a relative file path.
// Paths under x/<module>/ return the module name; others return the first path segment.
func ModuleFromPath(relPath string) string {
	parts := strings.Split(relPath, "/")
	if len(parts) >= 2 && parts[0] == "x" {
		return parts[1]
	}
	return parts[0]
}

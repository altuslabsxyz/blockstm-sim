//go:build !simharness

package simharness

import "github.com/altuslabsxyz/blockstm-sim/compare"

// RecordCall is a no-op when built without -tags simharness.
func RecordCall(category, callSite string) {}

// Provider returns nil — non-determinism detection is disabled without -tags simharness.
func Provider() compare.NonDetProvider { return nil }

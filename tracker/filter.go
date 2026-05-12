package tracker

import (
	"reflect"
	"strings"
)

// denyTrackerPrefixes lists keeper/module package path prefixes whose tracker
// instances should be skipped entirely. Use this for modules whose fields
// change deterministically (same value in oracle and probe) but are not
// KVStore-backed, causing false-positive out_of_kvstore findings.
//
// cosmossdk.io/x/upgrade: Keeper.downgradeVerified transitions false→true on
// the first block of every run. Both oracle and probe make the same transition
// so it never causes a real divergence, but the tracker records it as a
// mutation. Excluding the entire upgrade keeper avoids the false positive.
var denyTrackerPrefixes = []string{
	"cosmossdk.io/x/upgrade",
}

// ShouldSkipTracker returns true when a KeeperReflectTracker whose name starts
// with one of the denyTrackerPrefixes should be excluded from snapshotting.
func ShouldSkipTracker(name string) bool {
	for _, prefix := range denyTrackerPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// denyPkgPrefixes lists package path prefixes whose types are excluded from
// snapshotting: KVStore-backed types, immutable config, and framework internals
// that do not represent application-level mutable out-of-KV state.
var denyPkgPrefixes = []string{
	"cosmossdk.io/core/store",
	"cosmossdk.io/collections",
	"cosmossdk.io/schema",
	"cosmossdk.io/log",
	"cosmossdk.io/depinject",
	"github.com/cosmos/cosmos-sdk/codec",
	"github.com/cosmos/cosmos-sdk/baseapp",
	"github.com/cosmos/cosmos-sdk/runtime",
	"github.com/cosmos/cosmos-sdk/types/module",
	"github.com/cosmos/cosmos-sdk/client",
	"github.com/cosmos/cosmos-sdk/server",
	"github.com/cometbft/cometbft",
	"github.com/tendermint",
	"github.com/gogo/protobuf",
	"google.golang.org/grpc",
	"google.golang.org/protobuf",
}

// shouldSkipField returns true if the struct field should be excluded from
// snapshotting. It skips KV-backed types, immutable config, functions, channels,
// and interface fields (whose concrete types are unknown at compile time).
func shouldSkipField(f reflect.StructField) bool {
	ft := f.Type
	for ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}

	switch ft.Kind() {
	case reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return true
	case reflect.Interface:
		// Skip interfaces conservatively; concrete type is unknown.
		return true
	}

	pkg := ft.PkgPath()
	for _, prefix := range denyPkgPrefixes {
		if strings.HasPrefix(pkg, prefix) {
			return true
		}
	}
	return false
}

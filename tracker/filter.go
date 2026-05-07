package tracker

import (
	"reflect"
	"strings"
)

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
	for ft.Kind() == reflect.Ptr {
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

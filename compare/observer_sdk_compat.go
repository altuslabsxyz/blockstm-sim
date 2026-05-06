//go:build sdk_hooks

package compare

import "github.com/cosmos/cosmos-sdk/baseapp/lifecycle"

// Compile-time verification that BlockObserver still satisfies the SDK fork's
// lifecycle.LifecycleObserver via structural typing. If the SDK fork adds new
// methods to its interface, this file will fail to compile with a clear error.
var _ lifecycle.LifecycleObserver = (*BlockObserver)(nil)

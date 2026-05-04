package lifecycle_test

import (
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/stretchr/testify/assert"

	"github.com/cosmos/cosmos-sdk/baseapp/lifecycle"
)

var (
	_ lifecycle.LifecycleObserver = lifecycle.NoopLifecycleObserver{}
	_ lifecycle.LifecycleObserver = &lifecycle.NoopLifecycleObserver{}
)

func TestNoopLifecycleObserver_NoPanic(t *testing.T) {
	obs := lifecycle.NoopLifecycleObserver{}

	tests := []struct {
		name string
		fn   func()
	}{
		{"OnFinalizeBlockStart", func() { obs.OnFinalizeBlockStart(1) }},
		{"OnFinalizeBlockEnd", func() { obs.OnFinalizeBlockEnd([]byte("apphash")) }},
		{"OnTxStart", func() { obs.OnTxStart(0) }},
		{"OnTxEnd", func() { obs.OnTxEnd(0, &abci.ExecTxResult{}) }},
		{"OnTxEnd nil result", func() { obs.OnTxEnd(0, nil) }},
		{"OnKVWrite", func() { obs.OnKVWrite("bank", []byte("key"), 0) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, tc.fn)
		})
	}
}

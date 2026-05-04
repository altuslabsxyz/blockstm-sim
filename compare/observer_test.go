package compare_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/blockstm-sim/compare"
)

func TestBlockObserver_OnKVWrite(t *testing.T) {
	obs := compare.NewBlockObserver(3)

	obs.OnKVWrite("bank", []byte{0x01, 0x02}, 0)
	obs.OnKVWrite("bank", []byte{0x03, 0x04}, 0)
	obs.OnKVWrite("simcanary", []byte{0xAB}, 1)

	ws0 := obs.TxWriteSet(0)
	require.Equal(t, []string{"bank/0102", "bank/0304"}, ws0)

	ws1 := obs.TxWriteSet(1)
	require.Equal(t, []string{"simcanary/ab"}, ws1)

	ws2 := obs.TxWriteSet(2)
	require.Nil(t, ws2, "tx with no writes should return nil")
}

func TestBlockObserver_OnTxStart_Clears(t *testing.T) {
	obs := compare.NewBlockObserver(2)

	obs.OnKVWrite("bank", []byte{0x01}, 0)
	require.Len(t, obs.TxWriteSet(0), 1)

	obs.OnTxStart(0)
	require.Nil(t, obs.TxWriteSet(0), "OnTxStart must clear prior write set")
}

func TestBlockObserver_Dedup(t *testing.T) {
	obs := compare.NewBlockObserver(1)

	obs.OnKVWrite("bank", []byte{0x01}, 0)
	obs.OnKVWrite("bank", []byte{0x01}, 0)
	obs.OnKVWrite("bank", []byte{0x01}, 0)

	ws := obs.TxWriteSet(0)
	require.Equal(t, []string{"bank/01"}, ws, "duplicate keys must be deduplicated")
}

func TestBlockObserver_ConcurrentTxIndices(t *testing.T) {
	const txCount = 100
	obs := compare.NewBlockObserver(txCount)

	var wg sync.WaitGroup
	for i := 0; i < txCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			obs.OnKVWrite("bank", []byte{byte(idx)}, idx)
		}(i)
	}
	wg.Wait()

	for i := 0; i < txCount; i++ {
		ws := obs.TxWriteSet(i)
		require.Len(t, ws, 1, "each tx should have exactly one write")
	}
}

func TestBlockObserver_OutOfBounds(t *testing.T) {
	obs := compare.NewBlockObserver(2)

	obs.OnKVWrite("bank", []byte{0x01}, 99)
	obs.OnTxStart(99)
	require.Nil(t, obs.TxWriteSet(99))
}

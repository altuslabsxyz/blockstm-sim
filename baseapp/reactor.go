package baseapp

import (
	"fmt"
	"sync"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func (app *BaseApp) startReactors() {
	go app.checkTxAsyncReactor()
}

type RequestCheckTxAsync struct {
	txBytes  []byte
	txType   abci.CheckTxType
	callback abci.CheckTxCallback
	tx       sdk.Tx
	err      error

	// prepare is a WaitGroup used to ensure that transaction decoding and basic validation (preCheckTx)
	// is completed before the reactor processes the transaction.
	// This synchronization is necessary because:
	// 1. preCheckTx is executed in a separate goroutine for better performance
	// 2. The reactor must wait for this initial validation before proceeding with account-based ordering
	// 3. It prevents potential race conditions between transaction preparation and processing
	prepare *sync.WaitGroup
}

func (app *BaseApp) checkTxAsyncReactor() {
	for req := range app.chCheckTx {
		req.prepare.Wait()
		if req.err != nil {
			req.callback(sdkerrors.ResponseCheckTxWithEvents(req.err, 0, 0, nil, false))
			continue
		}

		waits, signals := app.checkAccountWGs.Register(app.cdc, req.tx)

		go app.checkTxAsync(req, waits, signals)
	}
}

func (app *BaseApp) prepareCheckTx(req *RequestCheckTxAsync) {
	defer req.prepare.Done()
	req.tx, req.err = app.txDecoder(req.txBytes)
}

func (app *BaseApp) checkTxAsync(req *RequestCheckTxAsync, waits []*sync.WaitGroup, signals []*AccountWG) {
	waitWgs(waits)
	defer app.checkAccountWGs.Done(signals)

	var mode sdk.ExecMode
	if req.txType == abci.CheckTxType_New {
		mode = execModeCheck
	} else if req.txType == abci.CheckTxType_Recheck {
		mode = execModeReCheck
	} else {
		panic(fmt.Sprintf("unknown RequestCheckTx type: %s", req.txType))
	}

	gInfo, result, anteEvents, err := app.RunTx(mode, req.txBytes, req.tx, -1, nil, nil)
	if err != nil {
		req.callback(sdkerrors.ResponseCheckTxWithEvents(err, gInfo.GasWanted, gInfo.GasUsed, anteEvents, app.trace))
		return
	}

	req.callback(&abci.ResponseCheckTx{
		GasWanted: int64(gInfo.GasWanted), // TODO: Should type accept unsigned ints?
		GasUsed:   int64(gInfo.GasUsed),   // TODO: Should type accept unsigned ints?
		Log:       result.Log,
		Data:      result.Data,
		Events:    sdk.MarkEventsToIndex(result.Events, app.indexEvents),
	})
}

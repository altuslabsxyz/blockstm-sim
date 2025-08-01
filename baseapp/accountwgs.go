package baseapp

import (
	"sync"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type AccountWGs struct {
	mtx sync.Mutex
	wgs map[string]*sync.WaitGroup
}

func NewAccountWGs() *AccountWGs {
	return &AccountWGs{
		wgs: make(map[string]*sync.WaitGroup),
	}
}

// Register registers transaction signers and returns necessary WaitGroups.
// It ensures that transactions from the same signer are processed sequentially.
//
// Parameters:
// - cdc: Codec for decoding transaction messages
// - tx: Transaction to be registered
//
// Returns:
// - waits: WaitGroups that need to be waited on (from previous transactions of the same signers)
// - signals: WaitGroups that should be signaled when this transaction completes
func (aw *AccountWGs) Register(cdc codec.Codec, tx sdk.Tx) (waits []*sync.WaitGroup, signals []*AccountWG) {
	// Get unique signers from the tx
	signers := getUniqSigners(cdc, tx)

	aw.mtx.Lock()
	defer aw.mtx.Unlock()
	for _, signer := range signers {
		// If there's an existing WaitGroup for this signer,
		// we need to wait for it before processing the current transaction
		if wg := aw.wgs[signer]; wg != nil {
			waits = append(waits, wg)
		}
		// Create a new WaitGroup for the current transaction
		sig := waitGroup1()
		aw.wgs[signer] = sig
		signals = append(signals, NewAccountWG(signer, sig))
	}

	return waits, signals
}

func (aw *AccountWGs) Done(signals []*AccountWG) {
	aw.mtx.Lock()
	defer aw.mtx.Unlock()

	for _, signal := range signals {
		signal.wg.Done()
		if aw.wgs[signal.acc] == signal.wg {
			delete(aw.wgs, signal.acc)
		}
	}
}

func getUniqSigners(cdc codec.Codec, tx sdk.Tx) (signers []string) {
	seen := map[string]bool{}
	for _, msg := range tx.GetMsgs() {
		msgSigners, _, _ := cdc.GetMsgV1Signers(msg)
		for _, msgSigner := range msgSigners {
			addr := sdk.AccAddress(msgSigner).String()
			if !seen[addr] {
				signers = append(signers, addr)
				seen[addr] = true
			}
		}
	}
	return signers
}

func waitWgs(waits []*sync.WaitGroup) {
	for _, wait := range waits {
		wait.Wait()
	}
}

type AccountWG struct {
	acc string
	wg  *sync.WaitGroup
}

func NewAccountWG(acc string, wg *sync.WaitGroup) *AccountWG {
	return &AccountWG{
		acc: acc,
		wg:  wg,
	}
}

func waitGroup1() (wg *sync.WaitGroup) {
	wg = &sync.WaitGroup{}
	wg.Add(1)
	return wg
}

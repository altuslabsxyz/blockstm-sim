package keeper

import (
	"context"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
)

// erc20Executor is a struct that houses ERC20 transfer and balanceOf executors.
// It exists so that the executors can be updated in the SendKeeper without needing to have a pointer receiver.
type erc20Executor struct {
	transfer  types.ERC20TransferFn
	balanceOf types.ERC20BalanceOfFn
}

// newERC20Executor creates a new erc20Executor with nil executors.
func newERC20Executor() *erc20Executor {
	return &erc20Executor{
		transfer:  nil,
		balanceOf: nil,
	}
}

// setTransfer sets the ERC20Transfer executor.
func (e *erc20Executor) setTransfer(transfer types.ERC20TransferFn) {
	e.transfer = transfer
}

// setBalanceOf sets the ERC20BalanceOf executor.
func (e *erc20Executor) setBalanceOf(balanceOf types.ERC20BalanceOfFn) {
	e.balanceOf = balanceOf
}

// executeTransfer executes the ERC20Transfer if there is one.
func (e *erc20Executor) ExecuteTransfer(ctx context.Context, contractAddr string, from, to sdk.Address, amount sdkmath.Int) error {
	if e == nil || e.transfer == nil {
		return nil
	}
	return e.transfer(ctx, contractAddr, from, to, amount)
}

// executeBalanceOf executes the ERC20BalanceOf if there is one.
func (e *erc20Executor) ExecuteBalanceOf(ctx context.Context, contractAddr string, addr sdk.Address) (sdkmath.Int, error) {
	if e == nil || e.balanceOf == nil {
		return sdkmath.Int{}, nil
	}
	return e.balanceOf(ctx, contractAddr, addr)
}

// SetERC20Transfer sets the ERC20Transfer executor.
func (k BaseSendKeeper) SetERC20Transfer(transfer types.ERC20TransferFn) {
	k.erc20Executor.setTransfer(transfer)
}

// SetERC20BalanceOf sets the ERC20BalanceOf executor.
func (k BaseSendKeeper) SetERC20BalanceOf(balanceOf types.ERC20BalanceOfFn) {
	k.erc20Executor.setBalanceOf(balanceOf)
}

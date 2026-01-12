package types

import (
	"context"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ERC20Transfer executes an ERC20 transfer operation.
// It transfers amount from the from address to the to address.
type ERC20TransferFn func(ctx context.Context, contractAddr string, from, to sdk.Address, amount sdkmath.Int) error

// ERC20BalanceOf executes an ERC20 balanceOf operation.
// It returns the balance of the specified address.
type ERC20BalanceOfFn func(ctx context.Context, contractAddr string, addr sdk.Address) (sdkmath.Int, error)

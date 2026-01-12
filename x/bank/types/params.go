package types

import (
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// DefaultDefaultSendEnabled is the value that DefaultSendEnabled will have from DefaultParams().
var DefaultDefaultSendEnabled = true

// NewParams creates a new parameter configuration for the bank module
func NewParams(defaultSendEnabled bool) Params {
	return Params{
		SendEnabled:        nil,
		DefaultSendEnabled: defaultSendEnabled,
		Erc20GasToken:      nil,
	}
}

// DefaultParams is the default parameter configuration for the bank module
func DefaultParams() Params {
	return Params{
		SendEnabled:        nil,
		DefaultSendEnabled: DefaultDefaultSendEnabled,
		Erc20GasToken:      nil,
	}
}

// Validate all bank module parameters
func (p Params) Validate() error {
	if len(p.SendEnabled) > 0 {
		return errors.New("use of send_enabled in params is no longer supported")
	}

	if p.Erc20GasToken != nil {
		if err := p.Erc20GasToken.Validate(); err != nil {
			return err
		}
	}

	return validateIsBool(p.DefaultSendEnabled)
}

// Validate gets any errors with this SendEnabled entry.
func (se SendEnabled) Validate() error {
	return sdk.ValidateDenom(se.Denom)
}

// NewSendEnabled creates a new SendEnabled object
// The denom may be left empty to control the global default setting of send_enabled
func NewSendEnabled(denom string, sendEnabled bool) *SendEnabled {
	return &SendEnabled{
		Denom:   denom,
		Enabled: sendEnabled,
	}
}

// Validate gets any errors with this ERC20GasToken entry.
func (e ERC20GasToken) Validate() error {
	if err := sdk.ValidateDenom(e.Denom); err != nil {
		return err
	}

	if !IsHexAddress(e.Erc20ContractAddress) {
		return fmt.Errorf("invalid ERC20 contract address: %s", e.Erc20ContractAddress)
	}

	return nil
}

// validateIsBool is used by the x/params module to validate that a thing is a bool.
func validateIsBool(i any) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

// Lengths of hashes and addresses in bytes.
const (
	// AddressLength is the expected length of the address
	AddressLength = 20
)

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// Ethereum address or not.
func IsHexAddress(s string) bool {
	if has0xPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*AddressLength && isHex(s)
}

// has0xPrefix validates str begins with '0x' or '0X'.
func has0xPrefix(str string) bool {
	return len(str) >= 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X')
}

// isHexCharacter returns bool of c being a valid hexadecimal.
func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// isHex validates whether each byte is valid hexadecimal string.
func isHex(str string) bool {
	if len(str)%2 != 0 {
		return false
	}
	for _, c := range []byte(str) {
		if !isHexCharacter(c) {
			return false
		}
	}
	return true
}

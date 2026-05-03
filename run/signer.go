package run

import (
	"crypto/sha256"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// DeriveKey returns a deterministic secp256k1 private key for the given
// account name. The same name always produces the same key and address.
func DeriveKey(name string) cryptotypes.PrivKey {
	seed := sha256.Sum256([]byte("blockstm-sim:" + name))
	return &secp256k1.PrivKey{Key: seed[:]}
}

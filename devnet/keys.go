package devnet

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"

	"github.com/dewnetwork/dew/crypto"
)

// Well-known Anvil / Hardhat private keys (dev only — never use on mainnet).
//
//	#0 faucet / ERC-20 deployer
//	#1 secondary user
//	#2 tertiary user
//	#3–#5 reserved validators when not overlapping with user roles
//
// Devnet validators use accounts #0, #1, #2 so a single keystore mental model
// matches MetaMask import of Anvil keys.
const (
	PrivHex0 = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	PrivHex1 = "59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d"
	PrivHex2 = "5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a"
	PrivHex3 = "7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6"
	PrivHex4 = "47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a"
	PrivHex5 = "8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba"

	AddrHex0 = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"
	AddrHex1 = "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
	AddrHex2 = "0x3C44CdDdB6a900fa2b585dd299e03d12FA4293BC"
)

// Account is a named dev keypair.
type Account struct {
	Name       string
	PrivateKey *ecdsa.PrivateKey
	Address    crypto.Address
	PrivHex    string
}

// MustAccount loads a hex private key or panics (dev fixtures only).
func MustAccount(name, privHex string) Account {
	a, err := AccountFromHex(name, privHex)
	if err != nil {
		panic(err)
	}
	return a
}

// AccountFromHex parses a 32-byte hex private key (no 0x).
func AccountFromHex(name, privHex string) (Account, error) {
	b, err := hex.DecodeString(privHex)
	if err != nil {
		return Account{}, fmt.Errorf("devnet: key %s: %w", name, err)
	}
	key, err := crypto.ToECDSA(b)
	if err != nil {
		return Account{}, fmt.Errorf("devnet: key %s: %w", name, err)
	}
	return Account{
		Name:       name,
		PrivateKey: key,
		Address:    crypto.PubkeyToAddress(&key.PublicKey),
		PrivHex:    privHex,
	}, nil
}

// Faucet is Anvil account #0 — pre-funded in genesis, used for ERC-20 deploy.
func Faucet() Account { return MustAccount("faucet", PrivHex0) }

// User1 is Anvil account #1 — transfer recipient in demos.
func User1() Account { return MustAccount("user1", PrivHex1) }

// User2 is Anvil account #2.
func User2() Account { return MustAccount("user2", PrivHex2) }

// DefaultValidators returns the three consensus validator accounts (#0–#2).
func DefaultValidators() []Account {
	return []Account{
		MustAccount("validator-0", PrivHex0),
		MustAccount("validator-1", PrivHex1),
		MustAccount("validator-2", PrivHex2),
	}
}

// DefaultUsers returns faucet + two funded EOAs for dapp demos.
func DefaultUsers() []Account {
	return []Account{Faucet(), User1(), User2()}
}

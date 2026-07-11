package devnet

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/crypto"
)

// DefaultChainID is the Dew local/test chain id (2205 / 0x89d).
const DefaultChainID int64 = 2205

// DefaultRPCPort is the JSON-RPC HTTP port for the RPC node.
const DefaultRPCPort = 8545

// DefaultP2PBasePort is the first P2P listen port (validators use base, base+1, …).
const DefaultP2PBasePort = 30303

// FaucetBalanceWei is 1_000_000 DEW (1e6 * 1e18) for each funded alloc.
var FaucetBalanceWei = new(big.Int).Mul(big.NewInt(1_000_000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))

// GenesisOptions customizes local genesis generation.
type GenesisOptions struct {
	ChainID    int64
	Validators []Account
	// Funded accounts (default: faucet + user1 + user2).
	Funded []Account
	// VotingPower per validator (default 1).
	VotingPower uint64
}

// BuildGenesis constructs a genesis document for local devnet.
func BuildGenesis(opts GenesisOptions) (*config.Genesis, error) {
	if opts.ChainID == 0 {
		opts.ChainID = DefaultChainID
	}
	if len(opts.Validators) == 0 {
		opts.Validators = DefaultValidators()
	}
	if len(opts.Validators) < 3 {
		return nil, fmt.Errorf("devnet: need at least 3 validators, got %d", len(opts.Validators))
	}
	if len(opts.Funded) == 0 {
		opts.Funded = DefaultUsers()
	}
	if opts.VotingPower == 0 {
		opts.VotingPower = 1
	}

	alloc := make(map[string]config.GenesisAccount, len(opts.Funded))
	bal := FaucetBalanceWei.String()
	for _, a := range opts.Funded {
		alloc[a.Address.Hex()] = config.GenesisAccount{Balance: bal}
	}
	// Ensure validators can pay gas if they also submit txs.
	for _, v := range opts.Validators {
		if _, ok := alloc[v.Address.Hex()]; !ok {
			alloc[v.Address.Hex()] = config.GenesisAccount{Balance: bal}
		}
	}

	ivals := make([]config.InitialValidator, len(opts.Validators))
	for i, v := range opts.Validators {
		pub := crypto.FromECDSAPub(&v.PrivateKey.PublicKey)
		ivals[i] = config.InitialValidator{
			Address:     v.Address.Hex(),
			PubKey:      "0x" + hex.EncodeToString(pub),
			VotingPower: opts.VotingPower,
		}
	}

	g := &config.Genesis{
		Config: &config.ChainConfig{
			ChainID:             big.NewInt(opts.ChainID),
			HomesteadBlock:      big.NewInt(0),
			EIP150Block:         big.NewInt(0),
			EIP155Block:         big.NewInt(0),
			EIP158Block:         big.NewInt(0),
			ByzantiumBlock:      big.NewInt(0),
			ConstantinopleBlock: big.NewInt(0),
			PetersburgBlock:     big.NewInt(0),
			IstanbulBlock:       big.NewInt(0),
			MuirGlacierBlock:    big.NewInt(0),
			BerlinBlock:         big.NewInt(0),
			LondonBlock:         big.NewInt(0),
			ShanghaiBlock:       big.NewInt(0),
			CancunBlock:         big.NewInt(0),
			Consensus: &config.ConsensusConfig{
				Type:                   "dew-bft",
				EpochLength:            86400,
				UnbondingPeriodSeconds: 604800,
				MinValidatorStake:      "100000000000000000000000",
				ActiveValidatorCap:     21,
			},
		},
		Nonce:             "0x0",
		Timestamp:         0,
		ExtraData:         "0x446577", // "Dew"
		GasLimit:          "0x7270e00",
		Difficulty:        "0x1",
		MixHash:           "0x0000000000000000000000000000000000000000000000000000000000000000",
		Coinbase:          "0x0000000000000000000000000000000000000000",
		BaseFeePerGas:     "0x3b9aca00",
		Alloc:             alloc,
		InitialValidators: ivals,
	}
	return g, nil
}

// WriteGenesisJSON writes pretty-printed genesis to path.
func WriteGenesisJSON(path string, g *config.Genesis) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o644)
}

// DefaultGenesis returns the standard 3-validator local genesis.
func DefaultGenesis() (*config.Genesis, error) {
	return BuildGenesis(GenesisOptions{})
}

// Package config holds chain genesis and node configuration.
package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/core/state"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

// ChainConfig is fork and consensus parameters from genesis.json.
type ChainConfig struct {
	ChainID             *big.Int         `json:"chainId"`
	HomesteadBlock      *big.Int         `json:"homesteadBlock"`
	EIP150Block         *big.Int         `json:"eip150Block"`
	EIP155Block         *big.Int         `json:"eip155Block"`
	EIP158Block         *big.Int         `json:"eip158Block"`
	ByzantiumBlock      *big.Int         `json:"byzantiumBlock"`
	ConstantinopleBlock *big.Int         `json:"constantinopleBlock"`
	PetersburgBlock     *big.Int         `json:"petersburgBlock"`
	IstanbulBlock       *big.Int         `json:"istanbulBlock"`
	MuirGlacierBlock    *big.Int         `json:"muirGlacierBlock"`
	BerlinBlock         *big.Int         `json:"berlinBlock"`
	LondonBlock         *big.Int         `json:"londonBlock"`
	ShanghaiBlock       *big.Int         `json:"shanghaiBlock"`
	CancunBlock         *big.Int         `json:"cancunBlock"`
	Consensus           *ConsensusConfig `json:"consensus,omitempty"`
}

// ConsensusConfig holds Dew-BFT parameters.
type ConsensusConfig struct {
	Type                   string `json:"type"`
	EpochLength            uint64 `json:"epochLength"`
	UnbondingPeriodSeconds uint64 `json:"unbondingPeriodSeconds"`
	MinValidatorStake      string `json:"minValidatorStake"`
	ActiveValidatorCap     uint64 `json:"activeValidatorCap"`
}

// GenesisAccount is one alloc entry.
type GenesisAccount struct {
	Balance string            `json:"balance"`
	Code    string            `json:"code,omitempty"`
	Storage map[string]string `json:"storage,omitempty"`
	Nonce   uint64            `json:"nonce,omitempty"`
}

// InitialValidator is a genesis validator set entry.
type InitialValidator struct {
	Address     string `json:"address"`
	PubKey      string `json:"pubKey"`
	VotingPower uint64 `json:"votingPower"`
}

// Genesis is the genesis.json schema.
type Genesis struct {
	Config            *ChainConfig               `json:"config"`
	Nonce             string                     `json:"nonce"`
	Timestamp         uint64                     `json:"timestamp"`
	ExtraData         string                     `json:"extraData"`
	GasLimit          string                     `json:"gasLimit"`
	Difficulty        string                     `json:"difficulty"`
	MixHash           string                     `json:"mixHash"`
	Coinbase          string                     `json:"coinbase"`
	BaseFeePerGas     string                     `json:"baseFeePerGas"`
	Alloc             map[string]GenesisAccount  `json:"alloc"`
	InitialValidators []InitialValidator         `json:"initialValidators"`
}

// LoadGenesisFile reads and parses a genesis JSON file.
func LoadGenesisFile(path string) (*Genesis, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read genesis: %w", err)
	}
	return ParseGenesis(raw)
}

// ParseGenesis decodes genesis JSON bytes.
func ParseGenesis(raw []byte) (*Genesis, error) {
	var g Genesis
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, fmt.Errorf("config: parse genesis: %w", err)
	}
	if g.Config == nil || g.Config.ChainID == nil {
		return nil, fmt.Errorf("config: genesis missing config.chainId")
	}
	if g.Alloc == nil {
		g.Alloc = map[string]GenesisAccount{}
	}
	return &g, nil
}

// ChainID returns the configured chain id.
func (g *Genesis) ChainID() *big.Int {
	if g.Config == nil || g.Config.ChainID == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(g.Config.ChainID)
}

// Commit applies alloc to a new StateDB on database and returns the genesis block
// with StateRoot set.
func (g *Genesis) Commit(database db.Database) (*types.Block, *state.StateDB, error) {
	statedb := state.New(database)
	if err := g.ApplyAlloc(statedb); err != nil {
		return nil, nil, err
	}
	root, err := statedb.Commit()
	if err != nil {
		return nil, nil, err
	}

	header, err := g.Header(root)
	if err != nil {
		return nil, nil, err
	}
	block := types.NewGenesisBlock(header)
	// NewGenesisBlock copies header; ensure state root retained
	block = block.WithStateRoot(root)
	return block, statedb, nil
}

// ApplyAlloc writes genesis accounts into statedb (does not commit).
func (g *Genesis) ApplyAlloc(statedb *state.StateDB) error {
	for addrStr, acc := range g.Alloc {
		addr, err := crypto.HexToAddress(addrStr)
		if err != nil {
			return fmt.Errorf("config: alloc address %q: %w", addrStr, err)
		}
		bal, ok := new(big.Int).SetString(strings.TrimSpace(acc.Balance), 0)
		if !ok {
			return fmt.Errorf("config: alloc %s: invalid balance %q", addrStr, acc.Balance)
		}
		if bal.Sign() < 0 {
			return fmt.Errorf("config: alloc %s: negative balance", addrStr)
		}
		statedb.SetBalance(addr, uint256.MustFromBig(bal))
		if acc.Nonce != 0 {
			statedb.SetNonce(addr, acc.Nonce)
		}
		if acc.Code != "" {
			code, err := parseHexBytes(acc.Code)
			if err != nil {
				return fmt.Errorf("config: alloc %s code: %w", addrStr, err)
			}
			statedb.SetCode(addr, code)
		}
		for slotStr, valStr := range acc.Storage {
			slot, err := types.HexToHash(slotStr)
			if err != nil {
				return fmt.Errorf("config: alloc %s storage key: %w", addrStr, err)
			}
			val, err := types.HexToHash(valStr)
			if err != nil {
				return fmt.Errorf("config: alloc %s storage val: %w", addrStr, err)
			}
			statedb.SetState(addr, slot, val)
		}
	}
	return nil
}

// Header builds the genesis header with the given state root.
func (g *Genesis) Header(stateRoot types.Hash) (*types.Header, error) {
	gasLimit := big.NewInt(120_000_000)
	if g.GasLimit != "" {
		v, err := parseUintHex(g.GasLimit)
		if err != nil {
			return nil, fmt.Errorf("config: gasLimit: %w", err)
		}
		gasLimit = v
	}
	baseFee := big.NewInt(1_000_000_000)
	if g.BaseFeePerGas != "" {
		v, err := parseUintHex(g.BaseFeePerGas)
		if err != nil {
			return nil, fmt.Errorf("config: baseFeePerGas: %w", err)
		}
		baseFee = v
	}
	extra, err := parseHexBytes(g.ExtraData)
	if err != nil {
		return nil, fmt.Errorf("config: extraData: %w", err)
	}
	return &types.Header{
		ParentHash:  types.Hash{},
		StateRoot:   stateRoot,
		TxRoot:      types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Number:      0,
		Timestamp:   g.Timestamp,
		GasLimit:    gasLimit.Uint64(),
		GasUsed:     0,
		BaseFee:     baseFee,
		ExtraData:   extra,
		Proposer:    types.EmptyProposer,
	}, nil
}

func parseUintHex(s string) (*big.Int, error) {
	s = strings.TrimSpace(s)
	v, ok := new(big.Int).SetString(s, 0)
	if !ok {
		return nil, fmt.Errorf("invalid integer %q", s)
	}
	return v, nil
}

func parseHexBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return hex.DecodeString(s)
}

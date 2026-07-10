package config

import (
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/db"
)

const sampleGenesis = `{
  "config": {
    "chainId": 2026,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "muirGlacierBlock": 0,
    "berlinBlock": 0,
    "londonBlock": 0,
    "shanghaiBlock": 0,
    "cancunBlock": 0,
    "consensus": {
      "type": "dew-bft",
      "epochLength": 86400,
      "unbondingPeriodSeconds": 604800,
      "minValidatorStake": "100000000000000000000000",
      "activeValidatorCap": 21
    }
  },
  "nonce": "0x0",
  "timestamp": 0,
  "extraData": "0x446577636861696e",
  "gasLimit": "0x7270e00",
  "difficulty": "0x1",
  "mixHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "coinbase": "0x0000000000000000000000000000000000000000",
  "baseFeePerGas": "0x3b9aca00",
  "alloc": {
    "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000000"
    },
    "0x70997970C51812dc3A010C7d01b50e0d17dc79C8": {
      "balance": "1000000000000000000000",
      "nonce": 1,
      "code": "0x6000",
      "storage": {
        "0x0000000000000000000000000000000000000000000000000000000000000001": "0x000000000000000000000000000000000000000000000000000000000000002a"
      }
    }
  },
  "initialValidators": []
}`

func TestParseGenesis_AndCommitAlloc(t *testing.T) {
	g, err := ParseGenesis([]byte(sampleGenesis))
	if err != nil {
		t.Fatalf("ParseGenesis: %v", err)
	}
	if g.ChainID().Cmp(big.NewInt(2026)) != 0 {
		t.Fatalf("chainId = %s", g.ChainID())
	}

	mdb := db.NewMemoryDB()
	defer mdb.Close()

	block, statedb, err := g.Commit(mdb)
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if block.Number() != 0 {
		t.Fatal("genesis height")
	}
	if block.Header().StateRoot.IsZero() {
		t.Fatal("zero state root")
	}
	if block.Header().GasLimit != 0x7270e00 {
		t.Fatalf("gasLimit = %d", block.Header().GasLimit)
	}
	if block.Header().BaseFee.Cmp(big.NewInt(0x3b9aca00)) != 0 {
		t.Fatalf("baseFee = %s", block.Header().BaseFee)
	}
	// ExtraData "Dewchain"
	if string(block.Header().ExtraData) != "Dewchain" {
		t.Fatalf("extraData = %q", block.Header().ExtraData)
	}

	// Alloc balances
	a0 := crypto.MustHexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	a1 := crypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	if statedb.GetBalance(a0).IsZero() {
		t.Fatal("alloc a0 balance zero")
	}
	if statedb.GetNonce(a1) != 1 {
		t.Fatalf("nonce a1 = %d", statedb.GetNonce(a1))
	}
	if len(statedb.GetCode(a1)) != 2 {
		t.Fatalf("code len = %d", len(statedb.GetCode(a1)))
	}
	slot, _ := types.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001")
	wantVal, _ := types.HexToHash("0x000000000000000000000000000000000000000000000000000000000000002a")
	if got := statedb.GetState(a1, slot); got != wantVal {
		t.Fatalf("storage = %s, want %s", got, wantVal)
	}

	// Header hash stable
	h1 := block.Hash()
	h2 := block.Header().Hash()
	if h1 != h2 || h1.IsZero() {
		t.Fatalf("header hash issue: %s %s", h1, h2)
	}

	// Second commit from same JSON → same state root + header hash
	mdb2 := db.NewMemoryDB()
	block2, _, err := g.Commit(mdb2)
	if err != nil {
		t.Fatal(err)
	}
	if block2.Header().StateRoot != block.Header().StateRoot {
		t.Fatalf("state root drift: %s vs %s", block.Header().StateRoot, block2.Header().StateRoot)
	}
	if block2.Hash() != block.Hash() {
		t.Fatalf("genesis hash drift: %s vs %s", block.Hash(), block2.Hash())
	}
}

func TestLoadGenesisFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "genesis.json")
	if err := os.WriteFile(path, []byte(sampleGenesis), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := LoadGenesisFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if g.ChainID().Int64() != 2026 {
		t.Fatal(g.ChainID())
	}
}

func TestGenesisHeader_HashFixture(t *testing.T) {
	g, err := ParseGenesis([]byte(sampleGenesis))
	if err != nil {
		t.Fatal(err)
	}
	mdb := db.NewMemoryDB()
	block, _, err := g.Commit(mdb)
	if err != nil {
		t.Fatal(err)
	}
	// Locked fixtures for sampleGenesis (updated for C3 SMT StateRoot).
	const (
		wantBlockHash = "0xa7fb91c47db4a0516fabf168f0015e402db22e52120b8970e72ae542b3bea06b"
		wantStateRoot = "0x423a426af8ad371a115388ac7704c6cbdcab52f310b6ebf5901654bac3e8e381"
	)
	if got := block.Hash().Hex(); got != wantBlockHash {
		t.Fatalf("genesis block hash = %s, want %s", got, wantBlockHash)
	}
	if got := block.Header().StateRoot.Hex(); got != wantStateRoot {
		t.Fatalf("genesis state root = %s, want %s", got, wantStateRoot)
	}
}

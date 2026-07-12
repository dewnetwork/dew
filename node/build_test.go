package node_test

import (
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
)

func TestBuildBlockFromPool_IncludesPendingTx(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)

	key, err := ethcrypto.HexToECDSA(devnet.PrivHex0)
	if err != nil {
		t.Fatal(err)
	}
	to := ethcrypto.PubkeyToAddress(key.PublicKey)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    0,
		GasPrice: big.NewInt(1_000_000_000),
		Gas:      21000,
		To:       &to,
		Value:    big.NewInt(1),
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(n.ChainID()), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}

	parent := n.CurrentHeader()
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, parent.Proposer, 1)
	if err != nil {
		t.Fatal(err)
	}
	if blk.Number() != 1 {
		t.Fatalf("number=%d", blk.Number())
	}
	if len(blk.Transactions()) != 1 {
		t.Fatalf("txs=%d", len(blk.Transactions()))
	}
	root, err := n.ValidateAndExecuteBlock(parent, blk)
	if err != nil {
		t.Fatal(err)
	}
	if root != blk.Header().StateRoot {
		t.Fatalf("root mismatch")
	}
}
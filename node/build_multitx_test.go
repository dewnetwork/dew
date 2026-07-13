package node_test

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	dewtypes "github.com/dewnetwork/dew/core/types"
	dewcrypto "github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
	"github.com/dewnetwork/dew/params"
)

func signDewTransfer(t *testing.T, privHex string, chainID *big.Int, nonce uint64, to dewcrypto.Address, amount *uint256.Int, fee uint64) []byte {
	t.Helper()
	keyBytes, err := hex.DecodeString(privHex)
	if err != nil {
		t.Fatal(err)
	}
	priv, err := dewcrypto.ToECDSA(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	from := ethcrypto.PubkeyToAddress(priv.PublicKey)
	var sender dewcrypto.Address
	copy(sender[:], from[:])
	tx := dewtypes.NewDewTx(chainID, nonce, sender, to, amount, fee, nil, nil)
	if err := dewtypes.SignDewTx(tx, priv); err != nil {
		t.Fatal(err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func signLegacyTransfer(t *testing.T, privHex string, chainID *big.Int, nonce uint64, to common.Address, value, gasPrice *big.Int) []byte {
	t.Helper()
	key, err := ethcrypto.HexToECDSA(privHex)
	if err != nil {
		t.Fatal(err)
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    nonce,
		GasPrice: gasPrice,
		Gas:      21_000,
		To:       &to,
		Value:    value,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(chainID), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestBuildBlockFromPool_MultiTxSameSender(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))
	n.SetAutoMine(false)

	user1, err := ethcrypto.HexToECDSA(devnet.PrivHex1)
	if err != nil {
		t.Fatal(err)
	}
	to := ethcrypto.PubkeyToAddress(user1.PublicKey)
	gp := big.NewInt(1_000_000_000)
	chainID := n.ChainID()

	// Three sequential nonces from Anvil #0 while auto-mine is off.
	for i := uint64(0); i < 3; i++ {
		raw := signLegacyTransfer(t, devnet.PrivHex0, chainID, i, to, big.NewInt(1), gp)
		if _, err := n.SendRawTransaction(raw); err != nil {
			t.Fatalf("admit nonce %d: %v", i, err)
		}
	}
	if got, _ := n.MempoolStats(); got != 3 {
		t.Fatalf("mempool len=%d want 3", got)
	}

	parent := n.CurrentHeader()
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, parent.Proposer, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(blk.Transactions()) != 3 {
		t.Fatalf("txs in block=%d want 3", len(blk.Transactions()))
	}
	// Nonce order preserved for same sender.
	for i, tx := range blk.Transactions() {
		if tx.Nonce != uint64(i) {
			t.Fatalf("tx[%d] nonce=%d want %d", i, tx.Nonce, i)
		}
	}
	root, err := n.ValidateAndExecuteBlock(parent, blk)
	if err != nil {
		t.Fatal(err)
	}
	if root != blk.Header().StateRoot {
		t.Fatalf("state root mismatch")
	}
}

func TestBuildBlockFromPool_PricePriorityAcrossSenders(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))
	n.SetAutoMine(false)

	// Recipient is Anvil #2 (unused sender).
	user2, err := ethcrypto.HexToECDSA(devnet.PrivHex2)
	if err != nil {
		t.Fatal(err)
	}
	to := ethcrypto.PubkeyToAddress(user2.PublicKey)
	chainID := n.ChainID()

	// Anvil #0: low tip (1 gwei), Anvil #1: high tip (2 gwei). Both nonce 0.
	low := signLegacyTransfer(t, devnet.PrivHex0, chainID, 0, to, big.NewInt(1), big.NewInt(1_000_000_000))
	high := signLegacyTransfer(t, devnet.PrivHex1, chainID, 0, to, big.NewInt(1), big.NewInt(2_000_000_000))
	if _, err := n.SendRawTransaction(low); err != nil {
		t.Fatal(err)
	}
	if _, err := n.SendRawTransaction(high); err != nil {
		t.Fatal(err)
	}

	parent := n.CurrentHeader()
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, parent.Proposer, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blk.Transactions()) != 1 {
		t.Fatalf("txs=%d want 1", len(blk.Transactions()))
	}
	// Higher gas price should win the single slot.
	got := blk.Transactions()[0].GasFeeCap
	if got == nil || got.Cmp(big.NewInt(2_000_000_000)) != 0 {
		t.Fatalf("expected high-priced tx selected, gasFeeCap=%v", got)
	}
}

func TestSendRawTransaction_NonceGapQueued(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))
	n.SetAutoMine(false)

	user1, err := ethcrypto.HexToECDSA(devnet.PrivHex1)
	if err != nil {
		t.Fatal(err)
	}
	to := ethcrypto.PubkeyToAddress(user1.PublicKey)
	chainID := n.ChainID()
	gp := big.NewInt(1_000_000_000)

	// Future nonce first (gap), then fill gap.
	raw1 := signLegacyTransfer(t, devnet.PrivHex0, chainID, 1, to, big.NewInt(1), gp)
	if _, err := n.SendRawTransaction(raw1); err != nil {
		t.Fatalf("future nonce admit: %v", err)
	}
	if got, _ := n.MempoolStats(); got != 1 {
		t.Fatalf("mempool=%d want 1", got)
	}

	raw0 := signLegacyTransfer(t, devnet.PrivHex0, chainID, 0, to, big.NewInt(1), gp)
	if _, err := n.SendRawTransaction(raw0); err != nil {
		t.Fatalf("fill gap: %v", err)
	}
	if got, _ := n.MempoolStats(); got != 2 {
		t.Fatalf("mempool=%d want 2", got)
	}

	parent := n.CurrentHeader()
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, parent.Proposer, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(blk.Transactions()) != 2 {
		t.Fatalf("txs=%d want 2", len(blk.Transactions()))
	}
}

func TestAutoMine_PacksReadyMultiTx(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))
	// autoMine default true

	user1, err := ethcrypto.HexToECDSA(devnet.PrivHex1)
	if err != nil {
		t.Fatal(err)
	}
	to := ethcrypto.PubkeyToAddress(user1.PublicKey)
	chainID := n.ChainID()
	gp := big.NewInt(1_000_000_000)

	// Queue future nonce first (stays pending under autoMine).
	raw1 := signLegacyTransfer(t, devnet.PrivHex0, chainID, 1, to, big.NewInt(1), gp)
	h1, err := n.SendRawTransaction(raw1)
	if err != nil {
		t.Fatal(err)
	}
	if n.GetReceipt(h1) != nil {
		t.Fatal("future nonce should not mine yet")
	}
	if n.BlockNumber() != 0 {
		t.Fatalf("height=%d want 0", n.BlockNumber())
	}

	// Exact nonce packs both into one block.
	raw0 := signLegacyTransfer(t, devnet.PrivHex0, chainID, 0, to, big.NewInt(1), gp)
	h0, err := n.SendRawTransaction(raw0)
	if err != nil {
		t.Fatal(err)
	}
	if n.BlockNumber() != 1 {
		t.Fatalf("height=%d want 1 after packing", n.BlockNumber())
	}
	if n.GetReceipt(h0) == nil || n.GetReceipt(h1) == nil {
		t.Fatal("both txs should have receipts after pack")
	}
	blk := n.GetBlockByNumber(1)
	if blk == nil || len(blk.Transactions()) != 2 {
		t.Fatalf("block txs=%v", blk)
	}
}

func TestAutoMine_DewTx_PacksReadyMultiTx(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))
	// native default on; autoMine default true

	to := dewcrypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	chainID := n.ChainID()
	fee := params.DefaultDewTxFeeWei
	amt := uint256.NewInt(1)

	// Queue future nonce first (stays pending under autoMine).
	raw1 := signDewTransfer(t, devnet.PrivHex0, chainID, 1, to, amt, fee)
	h1, err := n.SendDewRawTransaction(raw1)
	if err != nil {
		t.Fatal(err)
	}
	if n.GetReceipt(h1) != nil {
		t.Fatal("future nonce should not mine yet")
	}
	if n.BlockNumber() != 0 {
		t.Fatalf("height=%d want 0", n.BlockNumber())
	}
	if got, _ := n.MempoolStats(); got != 1 {
		t.Fatalf("mempool=%d want 1", got)
	}

	// Exact nonce packs both into one block.
	raw0 := signDewTransfer(t, devnet.PrivHex0, chainID, 0, to, amt, fee)
	h0, err := n.SendDewRawTransaction(raw0)
	if err != nil {
		t.Fatal(err)
	}
	if n.BlockNumber() != 1 {
		t.Fatalf("height=%d want 1 after packing", n.BlockNumber())
	}
	if n.GetReceipt(h0) == nil || n.GetReceipt(h1) == nil {
		t.Fatal("both DewTxs should have receipts after pack")
	}
	r0 := n.GetReceipt(h0)
	r1 := n.GetReceipt(h1)
	if r0.TransactionIndex != 0 || r1.TransactionIndex != 1 {
		t.Fatalf("tx indices: r0=%d r1=%d want 0,1", r0.TransactionIndex, r1.TransactionIndex)
	}
	if got, _ := n.MempoolStats(); got != 0 {
		t.Fatalf("mempool=%d want 0 after seal", got)
	}
	// Body remains empty (DewTx not in EVM body under public-testnet-v1).
	blk := n.GetBlockByNumber(1)
	if blk == nil || len(blk.Transactions()) != 0 {
		t.Fatalf("body txs=%v want empty", blk)
	}
}

func TestSendDewRawTransaction_NonceGapQueued(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))
	n.SetAutoMine(false)

	to := dewcrypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	chainID := n.ChainID()
	fee := params.DefaultDewTxFeeWei
	amt := uint256.NewInt(1)

	raw1 := signDewTransfer(t, devnet.PrivHex0, chainID, 1, to, amt, fee)
	if _, err := n.SendDewRawTransaction(raw1); err != nil {
		t.Fatalf("future nonce admit: %v", err)
	}
	raw0 := signDewTransfer(t, devnet.PrivHex0, chainID, 0, to, amt, fee)
	if _, err := n.SendDewRawTransaction(raw0); err != nil {
		t.Fatalf("fill gap: %v", err)
	}
	if got, _ := n.MempoolStats(); got != 2 {
		t.Fatalf("mempool=%d want 2", got)
	}
}

func TestSendDewRawTransaction_NonceTooLow(t *testing.T) {
	n := node.OpenTest(t, testGenesis(t))

	to := dewcrypto.MustHexToAddress("0x70997970C51812dc3A010C7d01b50e0d17dc79C8")
	chainID := n.ChainID()
	fee := params.DefaultDewTxFeeWei
	amt := uint256.NewInt(1)

	raw0 := signDewTransfer(t, devnet.PrivHex0, chainID, 0, to, amt, fee)
	if _, err := n.SendDewRawTransaction(raw0); err != nil {
		t.Fatal(err)
	}
	// Reuse nonce 0 after it was mined.
	rawDup := signDewTransfer(t, devnet.PrivHex0, chainID, 0, to, amt, fee)
	if _, err := n.SendDewRawTransaction(rawDup); err == nil {
		t.Fatal("expected nonce too low")
	}
}

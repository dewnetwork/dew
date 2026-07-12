package node_test

import (
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
)

func testGenesis(t *testing.T) *config.Genesis {
	t.Helper()
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestSetAutoMine_AdmitOnly(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)
	before := n.BlockNumber()

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
	if n.BlockNumber() != before {
		t.Fatalf("head advanced: %d -> %d", before, n.BlockNumber())
	}
	if n.Mempool().Len() != 1 {
		t.Fatalf("mempool len=%d want 1", n.Mempool().Len())
	}
}
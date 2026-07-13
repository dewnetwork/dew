package node_test

import (
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
)

func TestStack_CrossNodeTxBlockValidation(t *testing.T) {
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	a := node.OpenTest(t, g)
	b := node.OpenTest(t, g)
	a.SetAutoMine(false)
	b.SetAutoMine(false)

	key, err := ethcrypto.HexToECDSA(devnet.PrivHex0)
	if err != nil {
		t.Fatal(err)
	}
	to := ethcrypto.PubkeyToAddress(key.PublicKey)
	user1, err := ethcrypto.HexToECDSA(devnet.PrivHex1)
	if err != nil {
		t.Fatal(err)
	}
	recipient := ethcrypto.PubkeyToAddress(user1.PublicKey)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: 0, GasPrice: big.NewInt(1_000_000_000), Gas: 21_000,
		To: &recipient, Value: big.NewInt(1),
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(a.ChainID()), key)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := signed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SendRawTransaction(raw); err != nil {
		t.Fatal(err)
	}

	parent := a.CurrentHeader()
	vals := devnet.DefaultValidators()
	blk, err := a.BuildBlockFromPool(parent.Number+1, parent, vals[0].Address, 1)
	if err != nil {
		t.Fatal(err)
	}
	val := &consensus.ExecutionValidator{Exec: b}
	if err := val.ValidateProposal(parent.Number+1, parent, blk); err != nil {
		t.Fatalf("peer validation failed: %v", err)
	}
	_ = to
}
package node_test

import (
	"math/big"
	"strings"
	"testing"

	"encoding/hex"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/dewnetwork/dew/core/vm"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
)

func TestDeployBlock_BuildAndValidate(t *testing.T) {
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	n := node.OpenTest(t, g)
	n.SetAutoMine(false)
	faucet := devnet.Faucet()

	parsed, err := abi.JSON(strings.NewReader(vm.TokenABI))
	if err != nil {
		t.Fatal(err)
	}
	bin, err := hex.DecodeString(vm.TokenCreationBytecode)
	if err != nil {
		t.Fatal(err)
	}
	supply := new(big.Int).Mul(big.NewInt(1_000_000), big.NewInt(1e18))
	ctor, err := parsed.Pack("", supply)
	if err != nil {
		t.Fatal(err)
	}
	data := append(append([]byte{}, bin...), ctor...)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce: 0, GasPrice: big.NewInt(1_000_000_000), Gas: 3_000_000,
		Data: data,
	})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(n.ChainID()), faucet.PrivateKey)
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
	blk, err := n.BuildBlockFromPool(parent.Number+1, parent, faucet.Address, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(blk.Transactions()) != 1 {
		t.Fatalf("txs=%d", len(blk.Transactions()))
	}
}
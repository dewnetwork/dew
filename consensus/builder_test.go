package consensus_test

import (
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/dewnetwork/dew/config"
	"github.com/dewnetwork/dew/consensus"
	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/devnet"
	"github.com/dewnetwork/dew/node"
)

type fakeExec struct {
	node *node.Node
}

func (f *fakeExec) BuildBlockFromPool(height uint64, parent *types.Header, proposer crypto.Address, maxTxs int) (*types.Block, error) {
	return f.node.BuildBlockFromPool(height, parent, proposer, maxTxs)
}

func (f *fakeExec) ValidateAndExecuteBlock(parent *types.Header, block *types.Block) (types.Hash, error) {
	return f.node.ValidateAndExecuteBlock(parent, block)
}

func testGenesis(t *testing.T) *config.Genesis {
	t.Helper()
	g, err := devnet.DefaultGenesis()
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func admitTestTx(t *testing.T, n *node.Node) {
	t.Helper()
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
}

func TestMempoolBlockBuilder_BuildsBlock(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)
	admitTestTx(t, n)

	parent := n.CurrentHeader()
	builder := &consensus.MempoolBlockBuilder{
		Exec:   &fakeExec{node: n},
		MaxTxs: 1,
	}
	blk, err := builder.BuildProposal(parent.Number+1, parent, parent.Proposer, types.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	if blk.Number() != 1 {
		t.Fatalf("number=%d want 1", blk.Number())
	}
	if len(blk.Transactions()) != 1 {
		t.Fatalf("txs=%d want 1", len(blk.Transactions()))
	}
	if blk.Header().StateRoot.IsZero() {
		t.Fatal("expected non-zero state root")
	}
}

func TestExecutionValidator_AcceptsValidRoot(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)
	admitTestTx(t, n)

	parent := n.CurrentHeader()
	builder := &consensus.MempoolBlockBuilder{Exec: &fakeExec{node: n}, MaxTxs: 1}
	blk, err := builder.BuildProposal(parent.Number+1, parent, parent.Proposer, types.Hash{})
	if err != nil {
		t.Fatal(err)
	}

	val := &consensus.ExecutionValidator{Exec: &fakeExec{node: n}}
	if err := val.ValidateProposal(parent.Number+1, parent, blk); err != nil {
		t.Fatalf("valid block rejected: %v", err)
	}
}

func TestExecutionValidator_RejectsBadRoot(t *testing.T) {
	n, err := node.NewFromGenesis(testGenesis(t))
	if err != nil {
		t.Fatal(err)
	}
	n.SetAutoMine(false)
	admitTestTx(t, n)

	parent := n.CurrentHeader()
	builder := &consensus.MempoolBlockBuilder{Exec: &fakeExec{node: n}, MaxTxs: 1}
	blk, err := builder.BuildProposal(parent.Number+1, parent, parent.Proposer, types.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	bad := blk.WithStateRoot(types.Keccak256Hash([]byte("bad-root")))

	val := &consensus.ExecutionValidator{Exec: &fakeExec{node: n}}
	if err := val.ValidateProposal(parent.Number+1, parent, bad); err == nil {
		t.Fatal("expected rejection for bad state root")
	}
}

func TestValidatorSetFromGenesis(t *testing.T) {
	g := testGenesis(t)
	vs, err := consensus.ValidatorSetFromGenesis(g)
	if err != nil {
		t.Fatal(err)
	}
	if vs.Size() != len(g.InitialValidators) {
		t.Fatalf("size=%d want %d", vs.Size(), len(g.InitialValidators))
	}
	for _, iv := range g.InitialValidators {
		addr, err := crypto.HexToAddress(iv.Address)
		if err != nil {
			t.Fatal(err)
		}
		power := iv.VotingPower
		if power == 0 {
			power = 1
		}
		if vs.PowerOf(addr) != power {
			t.Fatalf("power for %s = %d want %d", iv.Address, vs.PowerOf(addr), power)
		}
	}
}
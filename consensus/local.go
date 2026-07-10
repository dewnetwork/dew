package consensus

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// LocalCluster runs N validators in-process with synchronous message fan-out.
// This exercises Dew-BFT without P2P (Phase A5).
type LocalCluster struct {
	mu      sync.Mutex
	engines []*Engine
	valSet  *ValidatorSet
	// commits by height
	commits map[uint64]*CommitEvent
	// optional: waiters
	commitCh chan CommitEvent
}

// LocalNode is one validator identity for cluster construction.
type LocalNode struct {
	Key         *ecdsa.PrivateKey
	Power       uint64
	ProposeRoot types.Hash
	Validator   ProposalValidator
}

// NewLocalCluster builds engines for each node and wires a shared broadcaster.
func NewLocalCluster(parent *types.Header, nodes []LocalNode) (*LocalCluster, error) {
	if parent == nil {
		return nil, fmt.Errorf("consensus: nil parent")
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("consensus: no nodes")
	}
	vals := make([]Validator, len(nodes))
	for i, n := range nodes {
		if n.Key == nil {
			return nil, fmt.Errorf("consensus: node %d missing key", i)
		}
		power := n.Power
		if power == 0 {
			power = 1
		}
		vals[i] = Validator{
			Address: crypto.PubkeyToAddress(&n.Key.PublicKey),
			Power:   power,
		}
	}
	valSet, err := NewValidatorSet(vals)
	if err != nil {
		return nil, err
	}

	c := &LocalCluster{
		engines:  make([]*Engine, 0, len(nodes)),
		valSet:   valSet,
		commits:  make(map[uint64]*CommitEvent),
		commitCh: make(chan CommitEvent, 64),
	}

	for _, n := range nodes {
		v := n.Validator
		if v == nil {
			v = NewBasicValidator()
		}
		eng, err := NewEngine(EngineConfig{
			PrivateKey:  n.Key,
			ValSet:      valSet,
			Parent:      parent,
			Builder:     &EmptyBlockBuilder{},
			Validator:   v,
			ProposeRoot: n.ProposeRoot,
		})
		if err != nil {
			return nil, err
		}
		height := parent.Number + 1 // capture for closure identity
		_ = height
		eng.OnCommit = func(ev CommitEvent) {
			c.onCommit(ev)
		}
		c.engines = append(c.engines, eng)
	}

	// Wire broadcasters after all engines exist.
	for _, eng := range c.engines {
		eng.SetBroadcaster(&clusterBus{cluster: c, from: eng.Address()})
	}
	return c, nil
}

type clusterBus struct {
	cluster *LocalCluster
	from    crypto.Address
}

func (b *clusterBus) BroadcastProposal(p *Proposal) {
	b.cluster.fanoutProposal(b.from, p)
}

func (b *clusterBus) BroadcastVote(v *Vote) {
	b.cluster.fanoutVote(b.from, v)
}

func (c *LocalCluster) fanoutProposal(from crypto.Address, p *Proposal) {
	for _, eng := range c.engines {
		if eng.Address().Equal(from) {
			continue // sender already applied
		}
		// Deliver synchronously; engine is re-entrant safe via its own mutex.
		_ = eng.HandleProposal(cloneProposal(p))
	}
}

func (c *LocalCluster) fanoutVote(from crypto.Address, v *Vote) {
	for _, eng := range c.engines {
		if eng.Address().Equal(from) {
			continue
		}
		_ = eng.HandleVote(cloneVote(v))
	}
}

func (c *LocalCluster) onCommit(ev CommitEvent) {
	c.mu.Lock()
	// First commit for this height wins (all honest should agree).
	if _, ok := c.commits[ev.Height]; !ok {
		cp := ev
		c.commits[ev.Height] = &cp
		select {
		case c.commitCh <- ev:
		default:
		}
	}
	c.mu.Unlock()
}

// Engines returns the validator engines (ordered as constructed).
func (c *LocalCluster) Engines() []*Engine { return c.engines }

// ValSet returns the shared validator set.
func (c *LocalCluster) ValSet() *ValidatorSet { return c.valSet }

// RunHeight starts a consensus round for the next height on every engine still
// at that height. With synchronous local broadcast, one proposer StartRound is
// usually enough for all honest nodes to commit.
//
// Returns the CommitEvent once a supermajority commit is observed.
func (c *LocalCluster) RunHeight() (*CommitEvent, error) {
	before := c.engines[0].Height()
	for _, eng := range c.engines {
		if eng.Height() != before {
			// Already advanced via commit delivered from a peer's round.
			continue
		}
		if err := eng.StartRound(); err != nil {
			return nil, err
		}
		// Proposer path often finalizes the height for the whole cluster.
		if c.CommitAt(before) != nil {
			break
		}
	}

	c.mu.Lock()
	ev, ok := c.commits[before]
	c.mu.Unlock()
	if !ok {
		for _, eng := range c.engines {
			if eng.LastCommit != nil && eng.LastCommit.Height == before {
				return eng.LastCommit, nil
			}
		}
		return nil, fmt.Errorf("consensus: height %d did not commit (nil polka / invalid proposal)", before)
	}
	return ev, nil
}

// CommitAt returns the commit event for height if recorded.
func (c *LocalCluster) CommitAt(height uint64) *CommitEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.commits[height]
}

// RunHeights commits n successive heights starting from the current next height.
func (c *LocalCluster) RunHeights(n int) ([]*CommitEvent, error) {
	out := make([]*CommitEvent, 0, n)
	for i := 0; i < n; i++ {
		ev, err := c.RunHeight()
		if err != nil {
			return out, err
		}
		out = append(out, ev)
	}
	return out, nil
}

// MakeGenesisHeader returns a minimal parent header for local tests.
func MakeGenesisHeader() *types.Header {
	return &types.Header{
		ParentHash:  types.Hash{},
		StateRoot:   types.Hash{},
		TxRoot:      types.EmptyTxRoot,
		ReceiptRoot: types.EmptyReceiptRoot,
		Number:      0,
		Timestamp:   1,
		GasLimit:    30_000_000,
		GasUsed:     0,
		BaseFee:     nil,
		ExtraData:   []byte("dew-bft-test"),
		Proposer:    types.EmptyProposer,
	}
}

// GenerateLocalNodes creates n keypairs with equal power and the same propose root.
func GenerateLocalNodes(n int, power uint64, proposeRoot types.Hash) ([]LocalNode, error) {
	if power == 0 {
		power = 1
	}
	nodes := make([]LocalNode, n)
	for i := 0; i < n; i++ {
		key, err := crypto.GenerateKey()
		if err != nil {
			return nil, err
		}
		nodes[i] = LocalNode{
			Key:         key,
			Power:       power,
			ProposeRoot: proposeRoot,
			Validator:   NewRootValidator(proposeRoot),
		}
	}
	return nodes, nil
}

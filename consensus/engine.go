package consensus

import (
	"crypto/ecdsa"
	"fmt"
	"sync"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Broadcaster sends consensus messages to peers (local bus or P2P later).
type Broadcaster interface {
	BroadcastProposal(p *Proposal)
	BroadcastVote(v *Vote)
}

// outbox collects messages while the engine lock is held; flushed after unlock
// so local synchronous delivery cannot deadlock on e.mu.
type outbox struct {
	proposals []*Proposal
	votes     []*Vote
	commit    *CommitEvent
}

func (o *outbox) proposal(p *Proposal) {
	if p != nil {
		o.proposals = append(o.proposals, cloneProposal(p))
	}
}

func (o *outbox) vote(v *Vote) {
	if v != nil {
		o.votes = append(o.votes, cloneVote(v))
	}
}

// Engine is one validator's Dew-BFT state machine for sequential heights.
type Engine struct {
	mu sync.Mutex

	address crypto.Address
	key     *ecdsa.PrivateKey
	valSet  *ValidatorSet

	height uint64
	round  uint64
	step   Step

	// parent is the last committed header (genesis or previous commit).
	parent *types.Header

	// proposal for current (height, round), if any
	proposal *Proposal

	prevotes   *voteSet
	precommits *voteSet

	// locked value (Tendermint-lite): once we precommit a block, remember it
	lockedRound int64
	lockedHash  types.Hash

	builder   BlockBuilder
	validator ProposalValidator
	// stateRoot used when this node proposes (claimed post-state).
	proposeRoot types.Hash

	bcast Broadcaster

	// committed this height (avoid double commit)
	committed bool
	// last commit for queries / tests
	LastCommit *CommitEvent

	// OnCommit is invoked (without holding mu) after a successful commit.
	OnCommit func(ev CommitEvent)
}

// EngineConfig wires a new Engine.
type EngineConfig struct {
	PrivateKey *ecdsa.PrivateKey
	ValSet     *ValidatorSet
	Parent     *types.Header // last committed header (genesis at start)
	Builder    BlockBuilder
	Validator  ProposalValidator
	// ProposeRoot is the state root this engine puts in its proposals.
	ProposeRoot types.Hash
	Broadcast   Broadcaster
}

// NewEngine constructs an engine starting at height parent.Number+1, round 0.
func NewEngine(cfg EngineConfig) (*Engine, error) {
	if cfg.PrivateKey == nil {
		return nil, fmt.Errorf("consensus: missing private key")
	}
	if cfg.ValSet == nil {
		return nil, fmt.Errorf("consensus: missing validator set")
	}
	if cfg.Parent == nil {
		return nil, fmt.Errorf("consensus: missing parent header")
	}
	addr := crypto.PubkeyToAddress(&cfg.PrivateKey.PublicKey)
	if _, ok := cfg.ValSet.Get(addr); !ok {
		return nil, fmt.Errorf("consensus: local address %s not in validator set", addr.Hex())
	}
	builder := cfg.Builder
	if builder == nil {
		builder = &EmptyBlockBuilder{}
	}
	validator := cfg.Validator
	if validator == nil {
		validator = NewBasicValidator()
	}
	e := &Engine{
		address:     addr,
		key:         cfg.PrivateKey,
		valSet:      cfg.ValSet,
		height:      cfg.Parent.Number + 1,
		round:       0,
		step:        StepNewRound,
		parent:      cfg.Parent.Copy(),
		builder:     builder,
		validator:   validator,
		proposeRoot: cfg.ProposeRoot,
		bcast:       cfg.Broadcast,
		lockedRound: -1,
	}
	e.resetVotes()
	return e, nil
}

// Address returns this validator's address.
func (e *Engine) Address() crypto.Address { return e.address }

// ValSet returns the current validator set (read-only snapshot under lock).
func (e *Engine) ValSet() *ValidatorSet {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.valSet
}

// ReplaceValSet swaps the active BFT set (D3c epoch rotation). Local engines
// whose address is no longer in the set stop proposing but still process messages.
func (e *Engine) ReplaceValSet(vs *ValidatorSet) error {
	if vs == nil || vs.Size() == 0 {
		return fmt.Errorf("consensus: cannot replace with empty validator set")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.valSet = vs
	return nil
}

// Height returns the current consensus height.
func (e *Engine) Height() uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.height
}

// Round returns the current round.
func (e *Engine) Round() uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.round
}

// Step returns the current step.
func (e *Engine) Step() Step {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.step
}

// SetBroadcaster updates the message sink (e.g. after LocalCluster wiring).
func (e *Engine) SetBroadcaster(b Broadcaster) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bcast = b
}

// SetProposeRoot updates the state root used when proposing.
func (e *Engine) SetProposeRoot(root types.Hash) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.proposeRoot = root
}

// SetValidator replaces the proposal validator (tests: inject root checker).
func (e *Engine) SetValidator(v ProposalValidator) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.validator = v
}

func (e *Engine) resetVotes() {
	e.prevotes = newVoteSet(VotePrevote, e.height, e.round)
	e.precommits = newVoteSet(VotePrecommit, e.height, e.round)
	e.proposal = nil
	e.committed = false
}

func (e *Engine) flush(box outbox) {
	if e.bcast != nil {
		for _, p := range box.proposals {
			e.bcast.BroadcastProposal(p)
		}
		for _, v := range box.votes {
			e.bcast.BroadcastVote(v)
		}
	}
	if box.commit != nil && e.OnCommit != nil {
		e.OnCommit(*box.commit)
	}
}

// StartRound enters Propose for the current height/round and may propose.
// Idempotent while a round is already in progress (step != NewRound).
//
// Block building runs without holding e.mu so it can take node locks without
// deadlocking against P2P import / ApplySyncedBlock paths.
func (e *Engine) StartRound() error {
	e.mu.Lock()
	if e.step != StepNewRound {
		e.mu.Unlock()
		return nil
	}
	e.resetVotes()
	e.step = StepPropose
	height, round := e.height, e.round
	parent := e.parent.Copy()
	addr := e.address
	root := e.proposeRoot
	isProposer := e.valSet.Proposer(height, round).Equal(addr)
	builder := e.builder
	key := e.key
	e.mu.Unlock()

	var block *types.Block
	if isProposer {
		var err error
		block, err = builder.BuildProposal(height, parent, addr, root)
		if err != nil {
			return err
		}
	}

	e.mu.Lock()
	var box outbox
	// Stale if height/round moved while building.
	if e.height != height || e.round != round || e.step != StepPropose {
		e.mu.Unlock()
		return nil
	}
	if !isProposer {
		// Non-proposer waits in Propose for HandleProposal.
		e.mu.Unlock()
		return nil
	}
	if block == nil {
		e.mu.Unlock()
		return fmt.Errorf("consensus: nil proposal block")
	}
	p := &Proposal{
		Height:    height,
		Round:     round,
		BlockHash: block.Hash(),
		Block:     block,
		Proposer:  addr,
	}
	if err := SignProposal(p, key); err != nil {
		e.mu.Unlock()
		return err
	}
	e.proposal = p
	box.proposal(p)
	// enterPrevote releases and re-acquires e.mu for execution validation.
	err := e.enterPrevoteLocked(&box)
	e.mu.Unlock()
	e.flush(box)
	return err
}

// HandleProposal processes a proposal from the network.
func (e *Engine) HandleProposal(p *Proposal) error {
	if p == nil {
		return fmt.Errorf("consensus: nil proposal")
	}
	if err := VerifyProposal(p); err != nil {
		return err
	}
	if p.Block != nil && p.Block.Hash() != p.BlockHash {
		return fmt.Errorf("consensus: proposal block hash mismatch")
	}

	e.mu.Lock()
	var box outbox
	if p.Height != e.height || p.Round != e.round {
		e.mu.Unlock()
		return nil // stale
	}
	if e.proposal != nil {
		e.mu.Unlock()
		return nil // already have one for this round
	}
	want := e.valSet.Proposer(e.height, e.round)
	if !p.Proposer.Equal(want) {
		e.mu.Unlock()
		return fmt.Errorf("consensus: proposal from non-proposer %s want %s", p.Proposer.Hex(), want.Hex())
	}
	e.proposal = cloneProposal(p)
	var err error
	if e.step == StepPropose || e.step == StepNewRound {
		err = e.enterPrevoteLocked(&box)
	}
	e.mu.Unlock()
	e.flush(box)
	return err
}

// enterPrevoteLocked transitions Propose → Prevote. Caller holds e.mu on entry;
// this method releases e.mu while validating the proposal against execution
// state (node locks), then re-acquires e.mu before recording the vote.
func (e *Engine) enterPrevoteLocked(box *outbox) error {
	if e.step != StepPropose && e.step != StepNewRound {
		return nil
	}
	e.step = StepPrevote

	height, round := e.height, e.round
	parent := e.parent.Copy()
	prop := cloneProposal(e.proposal)
	lockedRound := e.lockedRound
	lockedHash := e.lockedHash
	addr := e.address
	key := e.key
	validator := e.validator
	e.mu.Unlock()

	hash := types.Hash{} // nil by default
	if prop != nil && prop.Block != nil {
		if err := validator.ValidateProposal(height, parent, prop.Block); err == nil {
			hash = prop.BlockHash
		}
		// invalid root / linkage → nil prevote
	}
	if lockedRound >= 0 && !lockedHash.IsZero() {
		hash = lockedHash
	}

	v := &Vote{
		Type:      VotePrevote,
		Height:    height,
		Round:     round,
		BlockHash: hash,
		Validator: addr,
	}
	signErr := SignVote(v, key)

	e.mu.Lock()
	if signErr != nil {
		return signErr
	}
	// Stale if we moved on while validating.
	if e.height != height || e.round != round || e.step != StepPrevote {
		return nil
	}
	_ = e.addPrevoteLocked(v)
	box.vote(v)
	return e.tryAfterPrevoteLocked(box)
}

// HandleVote processes a prevote or precommit.
func (e *Engine) HandleVote(v *Vote) error {
	e.mu.Lock()
	var box outbox
	err := e.handleVoteLocked(v, &box)
	e.mu.Unlock()
	e.flush(box)
	return err
}

func (e *Engine) handleVoteLocked(v *Vote, box *outbox) error {
	if v == nil {
		return fmt.Errorf("consensus: nil vote")
	}
	if v.Height != e.height {
		return nil
	}
	if err := VerifyVote(v); err != nil {
		return err
	}
	if _, ok := e.valSet.Get(v.Validator); !ok {
		return fmt.Errorf("consensus: vote from non-validator %s", v.Validator.Hex())
	}

	switch v.Type {
	case VotePrevote:
		if v.Round != e.round {
			return nil
		}
		if !e.addPrevoteLocked(v) {
			return nil
		}
		return e.tryAfterPrevoteLocked(box)
	case VotePrecommit:
		if v.Round != e.round {
			return nil
		}
		if !e.addPrecommitLocked(v) {
			return nil
		}
		return e.tryAfterPrecommitLocked(box)
	default:
		return fmt.Errorf("consensus: unknown vote type")
	}
}

func (e *Engine) addPrevoteLocked(v *Vote) bool {
	power := e.valSet.PowerOf(v.Validator)
	return e.prevotes.add(v, power)
}

func (e *Engine) addPrecommitLocked(v *Vote) bool {
	power := e.valSet.PowerOf(v.Validator)
	return e.precommits.add(v, power)
}

func (e *Engine) tryAfterPrevoteLocked(box *outbox) error {
	if e.step != StepPrevote {
		return nil
	}
	maj, ok := e.prevotes.majorityHash(e.valSet.TotalPower())
	if !ok {
		return nil
	}
	return e.enterPrecommitLocked(maj, box)
}

func (e *Engine) enterPrecommitLocked(polkaHash types.Hash, box *outbox) error {
	e.step = StepPrecommit

	if !polkaHash.IsZero() {
		e.lockedRound = int64(e.round)
		e.lockedHash = polkaHash
	}

	v := &Vote{
		Type:      VotePrecommit,
		Height:    e.height,
		Round:     e.round,
		BlockHash: polkaHash,
		Validator: e.address,
	}
	if err := SignVote(v, e.key); err != nil {
		return err
	}
	_ = e.addPrecommitLocked(v)
	box.vote(v)
	return e.tryAfterPrecommitLocked(box)
}

func (e *Engine) tryAfterPrecommitLocked(box *outbox) error {
	if e.step != StepPrecommit {
		return nil
	}
	maj, ok := e.precommits.majorityHash(e.valSet.TotalPower())
	if !ok {
		return nil
	}
	if maj.IsZero() {
		// nil precommit quorum → prepare next round without auto-proposing.
		return e.enterNextRoundLocked()
	}
	return e.enterCommitLocked(maj, box)
}

func (e *Engine) enterNextRoundLocked() error {
	e.round++
	e.lockedRound = -1
	e.lockedHash = types.Hash{}
	e.resetVotes()
	e.step = StepNewRound
	return nil
}

func (e *Engine) enterCommitLocked(blockHash types.Hash, box *outbox) error {
	if e.committed {
		return nil
	}
	if e.proposal == nil || e.proposal.BlockHash != blockHash {
		return fmt.Errorf("consensus: commit hash %s without matching proposal", blockHash.Hex())
	}
	e.step = StepCommit
	e.committed = true

	ev := CommitEvent{
		Height:     e.height,
		Round:      e.round,
		BlockHash:  blockHash,
		Block:      e.proposal.Block,
		Precommits: e.precommits.votesFor(blockHash),
	}
	e.LastCommit = &ev

	if e.proposal.Block != nil {
		e.parent = e.proposal.Block.Header()
	}
	e.height = e.height + 1
	e.round = 0
	e.lockedRound = -1
	e.lockedHash = types.Hash{}
	e.resetVotes()
	e.step = StepNewRound

	cp := ev
	box.commit = &cp
	return nil
}

// ForceTimeoutRound advances to the next round (simulates timeout).
func (e *Engine) ForceTimeoutRound() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enterNextRoundLocked()
}

// ApplySyncedBlock advances the engine after a block was imported from P2P
// catch-up (validator missed the local BFT commit). Returns true when the
// engine moved forward; the caller should StartRound for the new height.
//
// No-op when the engine is already past block.Number (local commit won the race).
func (e *Engine) ApplySyncedBlock(block *types.Block) (bool, error) {
	if block == nil {
		return false, fmt.Errorf("consensus: nil block")
	}
	h := block.Header()
	if h == nil {
		return false, fmt.Errorf("consensus: nil header")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Already committed this height (or later) via local BFT.
	if e.height > h.Number {
		return false, nil
	}
	if e.height != h.Number {
		return false, fmt.Errorf("consensus: synced block height %d want %d", h.Number, e.height)
	}
	if e.parent == nil || h.ParentHash != e.parent.Hash() {
		return false, fmt.Errorf("consensus: synced block parent mismatch")
	}

	e.parent = h.Copy()
	e.height = h.Number + 1
	e.round = 0
	e.lockedRound = -1
	e.lockedHash = types.Hash{}
	e.committed = false
	e.resetVotes()
	e.step = StepNewRound
	e.LastCommit = &CommitEvent{
		Height:    h.Number,
		Round:     0,
		BlockHash: block.Hash(),
		Block:     block,
	}
	return true, nil
}

func cloneProposal(p *Proposal) *Proposal {
	if p == nil {
		return nil
	}
	cp := *p
	if p.Signature != nil {
		cp.Signature = append([]byte(nil), p.Signature...)
	}
	return &cp
}

func cloneVote(v *Vote) *Vote {
	if v == nil {
		return nil
	}
	cp := *v
	if v.Signature != nil {
		cp.Signature = append([]byte(nil), v.Signature...)
	}
	return &cp
}

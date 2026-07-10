package consensus

import (
	"fmt"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Step is the current round step in the Dew-BFT state machine.
type Step uint8

const (
	// StepNewRound waits to enter propose.
	StepNewRound Step = iota
	// StepPropose waits for (or creates) a proposal.
	StepPropose
	// StepPrevote has cast or is collecting prevotes.
	StepPrevote
	// StepPrecommit has cast or is collecting precommits.
	StepPrecommit
	// StepCommit has committed a block for this height.
	StepCommit
)

func (s Step) String() string {
	switch s {
	case StepNewRound:
		return "NewRound"
	case StepPropose:
		return "Propose"
	case StepPrevote:
		return "Prevote"
	case StepPrecommit:
		return "Precommit"
	case StepCommit:
		return "Commit"
	default:
		return fmt.Sprintf("Step(%d)", s)
	}
}

// CommitEvent is emitted when a height is finalized.
type CommitEvent struct {
	Height    uint64
	Round     uint64
	BlockHash types.Hash
	Block     *types.Block
	// Votes that formed the precommit quorum (copy).
	Precommits []Vote
}

// voteSet accumulates unique votes for one (height, round, type).
type voteSet struct {
	typ    VoteType
	height uint64
	round  uint64
	// votes by validator address
	byAddr map[crypto.Address]*Vote
	// power by block hash (zero hash = nil)
	power map[types.Hash]uint64
	// sum of all voting power seen
	totalSeen uint64
}

func newVoteSet(typ VoteType, height, round uint64) *voteSet {
	return &voteSet{
		typ:    typ,
		height: height,
		round:  round,
		byAddr: make(map[crypto.Address]*Vote),
		power:  make(map[types.Hash]uint64),
	}
}

// add records a verified vote from a set member. Returns false if duplicate or wrong slot.
func (vs *voteSet) add(v *Vote, power uint64) bool {
	if v.Type != vs.typ || v.Height != vs.height || v.Round != vs.round {
		return false
	}
	if _, exists := vs.byAddr[v.Validator]; exists {
		return false // one vote per validator per slot
	}
	cp := *v
	if v.Signature != nil {
		cp.Signature = append([]byte(nil), v.Signature...)
	}
	vs.byAddr[v.Validator] = &cp
	vs.power[v.BlockHash] += power
	vs.totalSeen += power
	return true
}

func (vs *voteSet) powerFor(hash types.Hash) uint64 {
	return vs.power[hash]
}

// majorityHash returns a hash that has quorum and true, if any.
// Prefers non-nil hashes; if only nil has quorum, returns zero hash.
func (vs *voteSet) majorityHash(totalPower uint64) (types.Hash, bool) {
	for h, p := range vs.power {
		if HasQuorum(p, totalPower) {
			return h, true
		}
	}
	return types.Hash{}, false
}

func (vs *voteSet) votesFor(hash types.Hash) []Vote {
	var out []Vote
	for _, v := range vs.byAddr {
		if v.BlockHash == hash {
			out = append(out, *v)
		}
	}
	return out
}

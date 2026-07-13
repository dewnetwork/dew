package consensus

import (
	"encoding/binary"
	"fmt"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// VoteWireSize is the fixed encoding length of one vote for precompile evidence.
// type(1) || height(8 BE) || round(8 BE) || blockHash(32) || signature(65) = 114.
const VoteWireSize = 1 + 8 + 8 + 32 + 65

// EncodeVoteWire serializes a vote for staking jail evidence (fixed 114 bytes).
func EncodeVoteWire(v *Vote) ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("consensus: nil vote")
	}
	if len(v.Signature) != 65 {
		return nil, fmt.Errorf("consensus: vote signature must be 65 bytes")
	}
	out := make([]byte, VoteWireSize)
	out[0] = byte(v.Type)
	binary.BigEndian.PutUint64(out[1:9], v.Height)
	binary.BigEndian.PutUint64(out[9:17], v.Round)
	copy(out[17:49], v.BlockHash[:])
	copy(out[49:114], v.Signature)
	return out, nil
}

// DecodeVoteWire parses a fixed 114-byte vote and recovers Validator from the signature.
func DecodeVoteWire(b []byte) (*Vote, error) {
	if len(b) != VoteWireSize {
		return nil, fmt.Errorf("consensus: vote wire length %d want %d", len(b), VoteWireSize)
	}
	v := &Vote{
		Type:   VoteType(b[0]),
		Height: binary.BigEndian.Uint64(b[1:9]),
		Round:  binary.BigEndian.Uint64(b[9:17]),
	}
	copy(v.BlockHash[:], b[17:49])
	v.Signature = append([]byte(nil), b[49:114]...)
	// Recover validator from signature so submitter cannot spoof the address field.
	digest, err := VoteSignBytes(v.Type, v.Height, v.Round, v.BlockHash)
	if err != nil {
		return nil, err
	}
	pub, err := crypto.Ecrecover(digest, v.Signature)
	if err != nil {
		return nil, fmt.Errorf("consensus: vote wire ecrecover: %w", err)
	}
	v.Validator = pubkeyBytesToAddress(pub)
	return v, nil
}

// EncodeDoubleSignEvidenceWire returns method-payload bytes: voteA || voteB (228 bytes).
func EncodeDoubleSignEvidenceWire(a, b *Vote) ([]byte, error) {
	wa, err := EncodeVoteWire(a)
	if err != nil {
		return nil, err
	}
	wb, err := EncodeVoteWire(b)
	if err != nil {
		return nil, err
	}
	return append(wa, wb...), nil
}

// DecodeDoubleSignEvidenceWire parses voteA||voteB (228 bytes).
func DecodeDoubleSignEvidenceWire(b []byte) (*DoubleSignEvidence, error) {
	if len(b) != 2*VoteWireSize {
		return nil, fmt.Errorf("consensus: evidence wire length %d want %d", len(b), 2*VoteWireSize)
	}
	a, err := DecodeVoteWire(b[:VoteWireSize])
	if err != nil {
		return nil, fmt.Errorf("vote A: %w", err)
	}
	bb, err := DecodeVoteWire(b[VoteWireSize:])
	if err != nil {
		return nil, fmt.Errorf("vote B: %w", err)
	}
	return &DoubleSignEvidence{VoteA: *a, VoteB: *bb}, nil
}

// DoubleSignEvidence proves one validator signed two conflicting votes
// for the same (type, height, round) with different block hashes.
// Used by the staking jail path (D3c) and can be gossiped as evidence later.
type DoubleSignEvidence struct {
	VoteA Vote
	VoteB Vote
}

// VerifyDoubleSign checks objective equivocation:
//   - both votes cryptographically valid
//   - same VoteType, Height, Round
//   - same recovered Validator address (and Vote.Validator fields match)
//   - distinct non-nil BlockHash values (nil vs non-nil or two different hashes)
func VerifyDoubleSign(a, b *Vote) error {
	if a == nil || b == nil {
		return fmt.Errorf("consensus: nil vote in evidence")
	}
	if err := VerifyVote(a); err != nil {
		return fmt.Errorf("consensus: evidence vote A: %w", err)
	}
	if err := VerifyVote(b); err != nil {
		return fmt.Errorf("consensus: evidence vote B: %w", err)
	}
	if a.Type != b.Type {
		return fmt.Errorf("consensus: evidence vote type mismatch %s vs %s", a.Type, b.Type)
	}
	if a.Type != VotePrevote && a.Type != VotePrecommit {
		return fmt.Errorf("consensus: evidence invalid vote type")
	}
	if a.Height != b.Height || a.Round != b.Round {
		return fmt.Errorf("consensus: evidence height/round mismatch (%d,%d) vs (%d,%d)",
			a.Height, a.Round, b.Height, b.Round)
	}
	if !a.Validator.Equal(b.Validator) {
		return fmt.Errorf("consensus: evidence validator mismatch %s vs %s",
			a.Validator.Hex(), b.Validator.Hex())
	}
	if a.BlockHash == b.BlockHash {
		return fmt.Errorf("consensus: evidence block hashes identical (not double-sign)")
	}
	// At least one non-zero hash is required so two pure nil votes cannot jail.
	if a.BlockHash.IsZero() && b.BlockHash.IsZero() {
		return fmt.Errorf("consensus: evidence both hashes nil")
	}
	return nil
}

// Verify reports whether e is valid double-sign evidence.
func (e *DoubleSignEvidence) Verify() error {
	if e == nil {
		return fmt.Errorf("consensus: nil evidence")
	}
	return VerifyDoubleSign(&e.VoteA, &e.VoteB)
}

// Offender returns the equivocating validator address (after Verify).
func (e *DoubleSignEvidence) Offender() crypto.Address {
	if e == nil {
		return crypto.Address{}
	}
	return e.VoteA.Validator
}

// EvidenceDigest is a stable 32-byte id for logging / optional precompile hash field.
func (e *DoubleSignEvidence) EvidenceDigest() types.Hash {
	if e == nil {
		return types.Hash{}
	}
	// Hash both vote sign digests so order of A/B is normalized by sorting bytes.
	da, _ := VoteSignBytes(e.VoteA.Type, e.VoteA.Height, e.VoteA.Round, e.VoteA.BlockHash)
	db, _ := VoteSignBytes(e.VoteB.Type, e.VoteB.Height, e.VoteB.Round, e.VoteB.BlockHash)
	if string(da) > string(db) {
		da, db = db, da
	}
	h := crypto.Keccak256(da, db, e.VoteA.Signature, e.VoteB.Signature)
	return types.BytesToHash(h)
}

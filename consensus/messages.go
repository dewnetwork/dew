package consensus

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// VoteType distinguishes prevote vs precommit.
type VoteType uint8

const (
	// VotePrevote is cast after proposal validation.
	VotePrevote VoteType = 1
	// VotePrecommit is cast after a prevote polka.
	VotePrecommit VoteType = 2
)

func (t VoteType) String() string {
	switch t {
	case VotePrevote:
		return "prevote"
	case VotePrecommit:
		return "precommit"
	default:
		return fmt.Sprintf("voteType(%d)", t)
	}
}

// Proposal is the proposer's signed block offer for (height, round).
type Proposal struct {
	Height    uint64
	Round     uint64
	BlockHash types.Hash
	// Block is optional body; validators may receive hash-only and fetch later.
	Block    *types.Block
	Proposer crypto.Address
	// Signature over SignBytes (65-byte ethereum [R||S||V]).
	Signature []byte
}

// Vote is a signed prevote or precommit. Zero BlockHash means nil vote.
type Vote struct {
	Type      VoteType
	Height    uint64
	Round     uint64
	BlockHash types.Hash // zero = nil
	Validator crypto.Address
	Signature []byte
}

// IsNil reports whether this is a nil vote.
func (v *Vote) IsNil() bool { return v.BlockHash.IsZero() }

// proposalSignPayload is the RLP-encoded material for proposal signatures.
type proposalSignPayload struct {
	Domain    string
	Height    uint64
	Round     uint64
	BlockHash []byte
}

// voteSignPayload is the RLP-encoded material for vote signatures.
type voteSignPayload struct {
	Domain    string
	Type      uint
	Height    uint64
	Round     uint64
	BlockHash []byte
}

const (
	proposalDomain = "Dew/Proposal/1"
	voteDomain     = "Dew/Vote/1"
)

// ProposalSignBytes returns the 32-byte digest to sign for a proposal.
func ProposalSignBytes(height, round uint64, blockHash types.Hash) ([]byte, error) {
	enc, err := rlp.EncodeToBytes(&proposalSignPayload{
		Domain:    proposalDomain,
		Height:    height,
		Round:     round,
		BlockHash: blockHash.Bytes(),
	})
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(enc), nil
}

// VoteSignBytes returns the 32-byte digest to sign for a vote.
func VoteSignBytes(typ VoteType, height, round uint64, blockHash types.Hash) ([]byte, error) {
	enc, err := rlp.EncodeToBytes(&voteSignPayload{
		Domain:    voteDomain,
		Type:      uint(typ),
		Height:    height,
		Round:     round,
		BlockHash: blockHash.Bytes(),
	})
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(enc), nil
}

// SignProposal fills Signature and Proposer from the private key.
func SignProposal(p *Proposal, key *ecdsa.PrivateKey) error {
	if p == nil {
		return fmt.Errorf("consensus: nil proposal")
	}
	digest, err := ProposalSignBytes(p.Height, p.Round, p.BlockHash)
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		return err
	}
	p.Signature = sig
	p.Proposer = crypto.PubkeyToAddress(&key.PublicKey)
	return nil
}

// VerifyProposal checks the proposer signature and that Proposer matches recovery.
func VerifyProposal(p *Proposal) error {
	if p == nil {
		return fmt.Errorf("consensus: nil proposal")
	}
	if len(p.Signature) != 65 {
		return fmt.Errorf("consensus: proposal signature must be 65 bytes")
	}
	digest, err := ProposalSignBytes(p.Height, p.Round, p.BlockHash)
	if err != nil {
		return err
	}
	pub, err := crypto.Ecrecover(digest, p.Signature)
	if err != nil {
		return fmt.Errorf("consensus: proposal ecrecover: %w", err)
	}
	// Ecrecover returns uncompressed 65-byte pubkey.
	addr := pubkeyBytesToAddress(pub)
	if !addr.Equal(p.Proposer) {
		return fmt.Errorf("consensus: proposal proposer mismatch: sig=%s field=%s", addr.Hex(), p.Proposer.Hex())
	}
	return nil
}

// SignVote fills Signature and Validator from the private key.
func SignVote(v *Vote, key *ecdsa.PrivateKey) error {
	if v == nil {
		return fmt.Errorf("consensus: nil vote")
	}
	digest, err := VoteSignBytes(v.Type, v.Height, v.Round, v.BlockHash)
	if err != nil {
		return err
	}
	sig, err := crypto.Sign(digest, key)
	if err != nil {
		return err
	}
	v.Signature = sig
	v.Validator = crypto.PubkeyToAddress(&key.PublicKey)
	return nil
}

// VerifyVote checks the vote signature.
func VerifyVote(v *Vote) error {
	if v == nil {
		return fmt.Errorf("consensus: nil vote")
	}
	if v.Type != VotePrevote && v.Type != VotePrecommit {
		return fmt.Errorf("consensus: invalid vote type %d", v.Type)
	}
	if len(v.Signature) != 65 {
		return fmt.Errorf("consensus: vote signature must be 65 bytes")
	}
	digest, err := VoteSignBytes(v.Type, v.Height, v.Round, v.BlockHash)
	if err != nil {
		return err
	}
	pub, err := crypto.Ecrecover(digest, v.Signature)
	if err != nil {
		return fmt.Errorf("consensus: vote ecrecover: %w", err)
	}
	addr := pubkeyBytesToAddress(pub)
	if !addr.Equal(v.Validator) {
		return fmt.Errorf("consensus: vote validator mismatch: sig=%s field=%s", addr.Hex(), v.Validator.Hex())
	}
	return nil
}

func pubkeyBytesToAddress(pub []byte) crypto.Address {
	// Uncompressed: 0x04 || X || Y (65 bytes) → Keccak(pub[1:])[12:]
	if len(pub) == 65 {
		h := crypto.Keccak256(pub[1:])
		var a crypto.Address
		copy(a[:], h[12:])
		return a
	}
	// Compressed 33-byte: use go-ethereum via ToECDSA path is heavier; require 65.
	var a crypto.Address
	if len(pub) >= 20 {
		copy(a[:], pub[len(pub)-20:])
	}
	return a
}

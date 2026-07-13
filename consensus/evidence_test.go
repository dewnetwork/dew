package consensus

import (
	"crypto/ecdsa"
	"testing"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

func testKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func signedVote(t *testing.T, key *ecdsa.PrivateKey, typ VoteType, h, r uint64, hash types.Hash) *Vote {
	t.Helper()
	v := &Vote{Type: typ, Height: h, Round: r, BlockHash: hash}
	if err := SignVote(v, key); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVerifyDoubleSign_OK(t *testing.T) {
	key := testKey(t)
	var ha, hb types.Hash
	ha[0], hb[0] = 0x01, 0x02
	a := signedVote(t, key, VotePrecommit, 10, 1, ha)
	b := signedVote(t, key, VotePrecommit, 10, 1, hb)
	if err := VerifyDoubleSign(a, b); err != nil {
		t.Fatal(err)
	}
	ev := &DoubleSignEvidence{VoteA: *a, VoteB: *b}
	if err := ev.Verify(); err != nil {
		t.Fatal(err)
	}
	if !ev.Offender().Equal(crypto.PubkeyToAddress(&key.PublicKey)) {
		t.Fatal("offender")
	}
	if ev.EvidenceDigest().IsZero() {
		t.Fatal("digest")
	}
}

func TestVerifyDoubleSign_RejectsSameHash(t *testing.T) {
	key := testKey(t)
	var h types.Hash
	h[0] = 0xab
	a := signedVote(t, key, VotePrecommit, 1, 0, h)
	b := signedVote(t, key, VotePrecommit, 1, 0, h)
	if err := VerifyDoubleSign(a, b); err == nil {
		t.Fatal("expected same-hash reject")
	}
}

func TestVerifyDoubleSign_RejectsDifferentValidators(t *testing.T) {
	k1, k2 := testKey(t), testKey(t)
	var ha, hb types.Hash
	ha[0], hb[0] = 1, 2
	a := signedVote(t, k1, VotePrevote, 5, 0, ha)
	b := signedVote(t, k2, VotePrevote, 5, 0, hb)
	if err := VerifyDoubleSign(a, b); err == nil {
		t.Fatal("expected different validators reject")
	}
}

func TestVerifyDoubleSign_RejectsHeightMismatch(t *testing.T) {
	key := testKey(t)
	var ha, hb types.Hash
	ha[0], hb[0] = 1, 2
	a := signedVote(t, key, VotePrecommit, 1, 0, ha)
	b := signedVote(t, key, VotePrecommit, 2, 0, hb)
	if err := VerifyDoubleSign(a, b); err == nil {
		t.Fatal("expected height mismatch reject")
	}
}

func TestVerifyDoubleSign_BadSignature(t *testing.T) {
	key := testKey(t)
	var ha, hb types.Hash
	ha[0], hb[0] = 1, 2
	a := signedVote(t, key, VotePrecommit, 1, 0, ha)
	b := signedVote(t, key, VotePrecommit, 1, 0, hb)
	b.Signature[0] ^= 0xff
	if err := VerifyDoubleSign(a, b); err == nil {
		t.Fatal("expected bad sig reject")
	}
}

func TestVoteWireRoundTripAndEvidence(t *testing.T) {
	key := testKey(t)
	var ha, hb types.Hash
	ha[0], hb[0] = 0x11, 0x22
	a := signedVote(t, key, VotePrecommit, 7, 2, ha)
	b := signedVote(t, key, VotePrecommit, 7, 2, hb)
	wire, err := EncodeDoubleSignEvidenceWire(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(wire) != 2*VoteWireSize {
		t.Fatalf("len %d", len(wire))
	}
	ev, err := DecodeDoubleSignEvidenceWire(wire)
	if err != nil {
		t.Fatal(err)
	}
	if err := ev.Verify(); err != nil {
		t.Fatal(err)
	}
}

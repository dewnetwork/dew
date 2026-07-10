package consensus

import (
	"testing"

	"github.com/dewnetwork/dew/crypto"
)

func TestHasQuorum(t *testing.T) {
	cases := []struct {
		vote, total uint64
		want        bool
	}{
		{0, 1, false},
		{1, 1, true},  // 3 > 2
		{1, 3, false}, // 3 > 6? no
		{2, 3, false}, // 6 > 6? no (strict)
		{3, 3, true},  // 9 > 6
		{2, 4, false}, // 6 > 8? no
		{3, 4, true},  // 9 > 8
		{67, 100, true},
		{66, 100, false}, // 198 > 200? no
	}
	for _, tc := range cases {
		if got := HasQuorum(tc.vote, tc.total); got != tc.want {
			t.Errorf("HasQuorum(%d,%d)=%v want %v", tc.vote, tc.total, got, tc.want)
		}
	}
}

func TestProposerDeterministicAndWeighted(t *testing.T) {
	a := crypto.MustHexToAddress("0x1111111111111111111111111111111111111111")
	b := crypto.MustHexToAddress("0x2222222222222222222222222222222222222222")
	c := crypto.MustHexToAddress("0x3333333333333333333333333333333333333333")
	vs, err := NewValidatorSet([]Validator{
		{Address: a, Power: 1},
		{Address: b, Power: 2},
		{Address: c, Power: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Same inputs → same proposer
	p1 := vs.Proposer(10, 0)
	p2 := vs.Proposer(10, 0)
	if p1 != p2 {
		t.Fatalf("non-deterministic proposer")
	}
	// Count proposals over many slots; B (power 2) should win ~half of total 4.
	counts := map[crypto.Address]int{}
	for h := uint64(0); h < 400; h++ {
		counts[vs.Proposer(h, 0)]++
	}
	if counts[b] < counts[a] || counts[b] < counts[c] {
		t.Fatalf("expected higher power to propose more often: a=%d b=%d c=%d", counts[a], counts[b], counts[c])
	}
	// Roughly half for B
	if counts[b] < 150 || counts[b] > 250 {
		t.Fatalf("unexpected B count %d (want ~200)", counts[b])
	}
}

func TestValidatorSetRejectsDuplicates(t *testing.T) {
	a := crypto.MustHexToAddress("0x1111111111111111111111111111111111111111")
	_, err := NewValidatorSet([]Validator{
		{Address: a, Power: 1},
		{Address: a, Power: 2},
	})
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

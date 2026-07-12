package consensus

import (
	"testing"
	"time"

	"github.com/dewnetwork/dew/core/types"
)

func TestDefaultMinBlockInterval(t *testing.T) {
	if defaultMinBlockInterval != time.Second {
		t.Fatalf("defaultMinBlockInterval=%v want 1s", defaultMinBlockInterval)
	}
}

// TestRunner_AdvancesAfterCommit — Runner OnCommit hook starts the next round.
func TestRunner_AdvancesAfterCommit(t *testing.T) {
	root := types.Keccak256Hash([]byte("runner"))
	nodes, err := GenerateLocalNodes(1, 1, root)
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := NewLocalCluster(MakeGenesisHeader(), nodes)
	if err != nil {
		t.Fatal(err)
	}
	eng := cluster.Engines()[0]

	var commitCount int
	runner := &Runner{
		Engine:           eng,
		RoundTimeout:     time.Hour, // disable timeout side-effects in this test
		MinBlockInterval: -1,        // immediate next round for unit test
		OnCommit: func(ev CommitEvent) error {
			commitCount++
			if ev.BlockHash.IsZero() {
				t.Error("empty block hash on commit")
			}
			return nil
		},
	}

	// Single-validator StartRound commits synchronously in a chain; run Start in
	// the background and observe that at least one commit advances the height.
	go func() {
		_ = runner.Start()
	}()

	deadline := time.Now().Add(200 * time.Millisecond)
	for commitCount == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	runner.Stop()

	if commitCount < 1 {
		t.Fatalf("commitCount=%d want >= 1", commitCount)
	}
	if h := eng.Height(); h < 2 {
		t.Fatalf("height=%d want >= 2 after commit + next StartRound", h)
	}
}
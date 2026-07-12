package p2p

import (
	"testing"
	"time"
)

func TestPeer_BulkSendDropsWhenQueueFull(t *testing.T) {
	// Peer with tiny outCh and no writeLoop consumer.
	p := &Peer{
		outCh:   make(chan outMsg, 1),
		closeCh: make(chan struct{}),
		host:    &Host{cfg: Config{WriteTimeout: 2 * time.Second}},
	}
	// Fill buffer.
	if err := p.Send(MsgInventory, []byte("a")); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	err := p.Send(MsgInventory, []byte("b")) // bulk — must not wait WriteTimeout
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("bulk drop should return nil, got %v", err)
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("bulk send blocked %v; want non-blocking drop", elapsed)
	}
}

func TestPeer_ConsensusSendWaitsWhenQueueFull(t *testing.T) {
	p := &Peer{
		outCh:   make(chan outMsg, 1),
		closeCh: make(chan struct{}),
		host:    &Host{cfg: Config{WriteTimeout: 50 * time.Millisecond}},
	}
	if err := p.Send(MsgProposal, []byte("a")); err != nil {
		t.Fatal(err)
	}
	// Unblock after short delay so we observe wait, not permanent hang.
	go func() {
		time.Sleep(20 * time.Millisecond)
		select {
		case <-p.outCh:
		default:
		}
	}()
	start := time.Now()
	err := p.Send(MsgProposal, []byte("b"))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("consensus send: %v", err)
	}
	if elapsed < 10*time.Millisecond {
		t.Fatalf("consensus send returned too fast (%v); expected short wait", elapsed)
	}
}

func TestIsConsensusMsg(t *testing.T) {
	if !IsConsensusMsg(MsgProposal) || !IsConsensusMsg(MsgPrevote) || !IsConsensusMsg(MsgPrecommit) {
		t.Fatal("expected consensus types")
	}
	if IsConsensusMsg(MsgInventory) || IsConsensusMsg(MsgTxPayload) {
		t.Fatal("inventory/tx must not be consensus class")
	}
}

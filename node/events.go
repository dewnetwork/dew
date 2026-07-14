package node

import (
	dewtypes "github.com/dewnetwork/dew/core/types"
)

// ChainEvent is emitted after a block is sealed or imported (for RPC subscriptions).
type ChainEvent struct {
	Header *dewtypes.Header
	Hash   dewtypes.Hash
	Logs   []*IndexedLog
}

const chainEventBuf = 64

type chainSub struct {
	id uint64
	ch chan ChainEvent
}

// SubscribeChainEvents registers a buffered listener for new heads + logs.
// Events are delivered best-effort (slow consumers may drop). Unsubscribe via the returned func.
func (n *Node) SubscribeChainEvents() (ch <-chan ChainEvent, unsubscribe func()) {
	n.eventMu.Lock()
	defer n.eventMu.Unlock()
	n.eventSeq++
	id := n.eventSeq
	c := make(chan ChainEvent, chainEventBuf)
	if n.eventSubs == nil {
		n.eventSubs = make(map[uint64]*chainSub)
	}
	n.eventSubs[id] = &chainSub{id: id, ch: c}
	return c, func() {
		n.eventMu.Lock()
		defer n.eventMu.Unlock()
		if s, ok := n.eventSubs[id]; ok {
			delete(n.eventSubs, id)
			close(s.ch)
		}
	}
}

// emitChainEventLocked notifies subscribers. Caller holds n.mu (commit path).
// Does not take n.mu; uses eventMu only. Callbacks are non-blocking channel sends.
func (n *Node) emitChainEventLocked(ev ChainEvent) {
	n.eventMu.Lock()
	subs := make([]*chainSub, 0, len(n.eventSubs))
	for _, s := range n.eventSubs {
		subs = append(subs, s)
	}
	n.eventMu.Unlock()

	for _, s := range subs {
		select {
		case s.ch <- ev:
		default:
			// drop if subscriber is slow
		}
	}
}

// event fields live on Node (see node.go).
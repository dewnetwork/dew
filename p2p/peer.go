package p2p

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dewnetwork/dew/core/types"
)

// outMsg is one queued outbound frame.
type outMsg struct {
	typ     uint8
	payload []byte
}

// Peer is one authenticated TCP session after handshake.
type Peer struct {
	ID         PeerID
	Conn       net.Conn
	RemoteAddr string // dialable host:port if known
	Height     uint64
	HeadHash   types.Hash
	Inbound    bool

	host    *Host
	secure  *secureConn // non-nil when C2 encryption is active
	sendMu  sync.Mutex  // only used by writeLoop
	outCh   chan outMsg
	closed  atomic.Bool
	closeCh chan struct{}
}

// Encrypted reports whether this peer session uses AES-GCM transport.
func (p *Peer) Encrypted() bool { return p.secure != nil }

func newPeer(h *Host, conn net.Conn, inbound bool) *Peer {
	return &Peer{
		Conn:    conn,
		Inbound: inbound,
		host:    h,
		outCh:   make(chan outMsg, 2048),
		closeCh: make(chan struct{}),
	}
}

// Send queues a framed message for the peer write loop. Consensus types wait
// up to WriteTimeout; bulk types drop immediately if the outbound queue is full.
func (p *Peer) Send(typ uint8, payload []byte) error {
	if p.closed.Load() {
		return fmt.Errorf("p2p: peer closed")
	}
	// Copy payload so callers can reuse buffers.
	msg := outMsg{typ: typ, payload: append([]byte(nil), payload...)}
	if IsConsensusMsg(typ) {
		return p.enqueueBlocking(msg)
	}
	return p.enqueueDrop(msg)
}

func (p *Peer) enqueueDrop(msg outMsg) error {
	select {
	case p.outCh <- msg:
		return nil
	case <-p.closeCh:
		return fmt.Errorf("p2p: peer closed")
	default:
		return nil // drop bulk under pressure
	}
}

func (p *Peer) enqueueBlocking(msg outMsg) error {
	timeout := 30 * time.Second
	if p.host != nil && p.host.cfg.WriteTimeout > 0 {
		timeout = p.host.cfg.WriteTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case p.outCh <- msg:
		return nil
	case <-p.closeCh:
		return fmt.Errorf("p2p: peer closed")
	case <-timer.C:
		return fmt.Errorf("p2p: outbound queue timeout")
	}
}

func (p *Peer) writeLoop() {
	for {
		select {
		case <-p.closeCh:
			return
		case msg := <-p.outCh:
			if err := p.writeFrame(msg.typ, msg.payload); err != nil {
				_ = p.Close()
				return
			}
		}
	}
}

func (p *Peer) writeFrame(typ uint8, payload []byte) error {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	if p.closed.Load() {
		return fmt.Errorf("p2p: peer closed")
	}
	_ = p.Conn.SetWriteDeadline(time.Now().Add(p.host.cfg.WriteTimeout))
	if p.secure != nil {
		return p.secure.WriteFrame(typ, payload)
	}
	return WriteFrame(p.Conn, typ, payload, p.host.cfg.MaxMsgSize)
}

// Close terminates the peer connection.
func (p *Peer) Close() error {
	if p.closed.Swap(true) {
		return nil
	}
	close(p.closeCh)
	return p.Conn.Close()
}

// runRead loops reading frames until error or close.
func (p *Peer) runRead() {
	defer func() {
		_ = p.Close()
		p.host.onPeerClosed(p)
	}()
	for {
		select {
		case <-p.closeCh:
			return
		default:
		}
		_ = p.Conn.SetReadDeadline(time.Now().Add(p.host.cfg.ReadTimeout))
		var (
			frame Frame
			err   error
		)
		if p.secure != nil {
			frame, err = p.secure.ReadFrame()
		} else {
			frame, err = ReadFrame(p.Conn, p.host.cfg.MaxMsgSize)
		}
		if err != nil {
			return
		}
		if err := p.host.handleFrame(p, frame); err != nil {
			// Protocol error: drop peer.
			return
		}
	}
}

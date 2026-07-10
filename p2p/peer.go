package p2p

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dewnetwork/dew/core/types"
)

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
	sendMu  sync.Mutex
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
		closeCh: make(chan struct{}),
	}
}

// Send writes a framed message to the peer (serialized).
func (p *Peer) Send(typ uint8, payload []byte) error {
	if p.closed.Load() {
		return fmt.Errorf("p2p: peer closed")
	}
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
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

package p2p

import (
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
)

// Config configures a P2P host.
type Config struct {
	PrivateKey  *ecdsa.PrivateKey
	ChainID     *big.Int
	ListenAddr  string // e.g. "127.0.0.1:0"
	MaxPeers    int
	MaxMsgSize  int
	ReadTimeout time.Duration
	WriteTimeout time.Duration
	// MaxBlocksPerRequest caps GetBlocks range size.
	MaxBlocksPerRequest uint64
	// Encrypt enables X25519 + AES-256-GCM session wrapping before the
	// identity handshake. Default true (Phase C2). Set false only for
	// explicit cleartext dev/loopback (AllowCleartext must also be true).
	Encrypt bool
	// AllowCleartext permits Encrypt=false. Without this flag, NewHost
	// rejects cleartext config so public/multi-host nets fail closed.
	AllowCleartext bool
}

// Host is a P2P node: listen, dial, handshake, gossip, sync, consensus fan-out.
type Host struct {
	cfg      Config
	nodeID   PeerID
	store    *PeerStore
	chain    ChainBackend
	txs      TxBackend
	handlers AppHandlers

	ln       net.Listener
	listenPort uint16

	mu     sync.Mutex
	closed atomic.Bool
	wg     sync.WaitGroup

	// seenInv avoids re-requesting the same inventory item.
	seenMu sync.Mutex
	seenInv map[types.Hash]struct{}
}

// NewHost builds a host. Call Start to listen.
func NewHost(cfg Config, chain ChainBackend, txs TxBackend, handlers AppHandlers) (*Host, error) {
	if cfg.PrivateKey == nil {
		return nil, fmt.Errorf("p2p: missing private key")
	}
	if cfg.ChainID == nil {
		return nil, fmt.Errorf("p2p: missing chain id")
	}
	if chain == nil {
		return nil, fmt.Errorf("p2p: missing chain backend")
	}
	if txs == nil {
		txs = &emptyTxBackend{}
	}
	if cfg.MaxMsgSize <= 0 {
		cfg.MaxMsgSize = DefaultMaxMsgSize
	}
	if cfg.MaxPeers <= 0 {
		cfg.MaxPeers = 25
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 60 * time.Second
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.MaxBlocksPerRequest == 0 {
		cfg.MaxBlocksPerRequest = 100
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:0"
	}
	// Default Encrypt=true unless caller explicitly requested cleartext.
	// Zero-value Config gets encryption (safe default for multi-host).
	if !cfg.Encrypt && !cfg.AllowCleartext {
		cfg.Encrypt = true
	}
	if !cfg.Encrypt && !cfg.AllowCleartext {
		return nil, fmt.Errorf("p2p: cleartext requires AllowCleartext=true")
	}
	return &Host{
		cfg:     cfg,
		nodeID:  crypto.PubkeyToAddress(&cfg.PrivateKey.PublicKey),
		store:   NewPeerStore(cfg.MaxPeers),
		chain:   chain,
		txs:     txs,
		handlers: handlers,
		seenInv: make(map[types.Hash]struct{}),
	}, nil
}

// EncryptEnabled reports whether new sessions use encrypted transport.
func (h *Host) EncryptEnabled() bool { return h.cfg.Encrypt }

type emptyTxBackend struct{}

func (emptyTxBackend) HasTx(types.Hash) bool            { return false }
func (emptyTxBackend) GetTx(types.Hash) ([]byte, bool)  { return nil, false }

// ID returns this node's peer id.
func (h *Host) ID() PeerID { return h.nodeID }

// Store returns the peer store.
func (h *Host) Store() *PeerStore { return h.store }

// ListenAddr returns the bound address after Start (empty if not listening).
func (h *Host) ListenAddr() string {
	if h.ln == nil {
		return ""
	}
	return h.ln.Addr().String()
}

// ListenPort returns the bound TCP port.
func (h *Host) ListenPort() uint16 { return h.listenPort }

// Start begins accepting inbound connections.
func (h *Host) Start() error {
	if h.closed.Load() {
		return fmt.Errorf("p2p: host closed")
	}
	ln, err := net.Listen("tcp", h.cfg.ListenAddr)
	if err != nil {
		return err
	}
	h.ln = ln
	// Extract port
	if ta, ok := ln.Addr().(*net.TCPAddr); ok {
		h.listenPort = uint16(ta.Port)
	}
	h.wg.Add(1)
	go h.acceptLoop()
	return nil
}

// Close stops the listener and all peers.
func (h *Host) Close() error {
	if h.closed.Swap(true) {
		return nil
	}
	if h.ln != nil {
		_ = h.ln.Close()
	}
	for _, p := range h.store.Active() {
		_ = p.Close()
	}
	h.wg.Wait()
	return nil
}

func (h *Host) acceptLoop() {
	defer h.wg.Done()
	for {
		conn, err := h.ln.Accept()
		if err != nil {
			if h.closed.Load() {
				return
			}
			continue
		}
		h.wg.Add(1)
		go func(c net.Conn) {
			defer h.wg.Done()
			if _, err := h.negotiate(c, true); err != nil {
				_ = c.Close()
			}
		}(conn)
	}
}

// Dial connects outbound to addr (host:port), performs handshake, and registers the peer.
func (h *Host) Dial(addr string) (*Peer, error) {
	if h.closed.Load() {
		return nil, fmt.Errorf("p2p: host closed")
	}
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, err
	}
	p, err := h.negotiate(conn, false)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return p, nil
}

func (h *Host) negotiate(conn net.Conn, inbound bool) (*Peer, error) {
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	defer conn.SetDeadline(time.Time{})

	var (
		writeFn func(typ uint8, payload []byte) error
		readFn  func() (Frame, error)
		sec     *secureConn
	)
	if h.cfg.Encrypt {
		sess, err := performSecureHandshake(conn, h.cfg.ChainID, h.cfg.MaxMsgSize, inbound)
		if err != nil {
			return nil, err
		}
		sec = &secureConn{Conn: conn, sess: sess, maxSize: h.cfg.MaxMsgSize}
		writeFn = sec.WriteFrame
		readFn = sec.ReadFrame
	} else {
		writeFn = func(typ uint8, payload []byte) error {
			return WriteFrame(conn, typ, payload, h.cfg.MaxMsgSize)
		}
		readFn = func() (Frame, error) {
			return ReadFrame(conn, h.cfg.MaxMsgSize)
		}
	}

	local, err := h.buildHandshake()
	if err != nil {
		return nil, err
	}
	payload, err := local.Encode()
	if err != nil {
		return nil, err
	}

	// Simultaneous exchange: write in a goroutine to avoid deadlock.
	errCh := make(chan error, 1)
	go func() {
		errCh <- writeFn(MsgHandshake, payload)
	}()
	frame, err := readFn()
	if err != nil {
		return nil, fmt.Errorf("p2p: read handshake: %w", err)
	}
	if werr := <-errCh; werr != nil {
		return nil, fmt.Errorf("p2p: write handshake: %w", werr)
	}
	if frame.Type != MsgHandshake {
		return nil, fmt.Errorf("p2p: expected handshake, got 0x%02x", frame.Type)
	}
	remote, err := DecodeHandshake(frame.Payload)
	if err != nil {
		return nil, err
	}
	if err := h.verifyHandshake(remote); err != nil {
		return nil, err
	}
	if remote.NodeID.Equal(h.nodeID) {
		return nil, fmt.Errorf("p2p: connected to self")
	}

	p := newPeer(h, conn, inbound)
	p.secure = sec
	p.ID = remote.NodeID
	p.Height = remote.Height
	p.HeadHash = remote.HeadHash
	if host, _, err := net.SplitHostPort(conn.RemoteAddr().String()); err == nil && remote.ListenPort > 0 {
		p.RemoteAddr = FormatAddr(host, remote.ListenPort)
	} else {
		p.RemoteAddr = conn.RemoteAddr().String()
	}

	if err := h.store.AddActive(p); err != nil {
		return nil, err
	}
	h.store.Remember(p.ID, p.RemoteAddr)

	h.wg.Add(2)
	go func() {
		defer h.wg.Done()
		p.writeLoop()
	}()
	go func() {
		defer h.wg.Done()
		p.runRead()
	}()
	return p, nil
}

func (h *Host) buildHandshake() (*Handshake, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	hs := &Handshake{
		Version:    ProtocolVersion,
		ChainID:    new(big.Int).Set(h.cfg.ChainID),
		Height:     h.chain.Height(),
		HeadHash:   h.chain.HeadHash(),
		ListenPort: h.listenPort,
		NodeID:     h.nodeID,
		Nonce:      nonce,
	}
	digest, err := hs.SignBytes()
	if err != nil {
		return nil, err
	}
	sig, err := crypto.Sign(digest, h.cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	hs.Signature = sig
	return hs, nil
}

func (h *Host) verifyHandshake(hs *Handshake) error {
	if hs.Version != ProtocolVersion {
		return fmt.Errorf("p2p: protocol version mismatch: got %d want %d", hs.Version, ProtocolVersion)
	}
	if hs.ChainID == nil || hs.ChainID.Cmp(h.cfg.ChainID) != 0 {
		return fmt.Errorf("p2p: chain id mismatch")
	}
	if len(hs.Signature) != 65 || len(hs.Nonce) == 0 {
		return fmt.Errorf("p2p: invalid handshake signature/nonce")
	}
	digest, err := hs.SignBytes()
	if err != nil {
		return err
	}
	pub, err := crypto.Ecrecover(digest, hs.Signature)
	if err != nil {
		return fmt.Errorf("p2p: handshake ecrecover: %w", err)
	}
	if len(pub) != 65 {
		return fmt.Errorf("p2p: bad recovered pubkey")
	}
	addrHash := crypto.Keccak256(pub[1:])
	var addr PeerID
	copy(addr[:], addrHash[12:])
	if !addr.Equal(hs.NodeID) {
		return fmt.Errorf("p2p: handshake node id does not match signature")
	}
	return nil
}

func (h *Host) onPeerClosed(p *Peer) {
	h.store.RemoveActive(p.ID)
}

// Broadcast sends a framed message to all active peers except exclude (zero = none).
func (h *Host) Broadcast(typ uint8, payload []byte, exclude PeerID) {
	for _, p := range h.store.Active() {
		if !exclude.IsZero() && p.ID.Equal(exclude) {
			continue
		}
		_ = p.Send(typ, payload)
	}
}

// PeerCount returns active peers.
func (h *Host) PeerCount() int { return h.store.ActiveCount() }

package p2p

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"net"
	"sync"

	"github.com/dewnetwork/dew/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// MsgSecureHello is exchanged before the authenticated identity handshake when
// encryption is enabled. Cipher suite: X25519 ECDH + AES-256-GCM.
// Framing after the hello remains length-prefixed; the body is ciphertext.
const MsgSecureHello uint8 = 0x00

// SecureVersion is the encrypted transport handshake version.
const SecureVersion uint32 = 1

// secureHelloPayload is the cleartext body of MsgSecureHello.
type secureHelloPayload struct {
	Version uint32
	EphPub  []byte // 32-byte X25519 public key
	Random  []byte // 32-byte nonce
}

// session holds directional AES-GCM sealers and 64-bit message counters.
// Send and recv use separate mutexes so handshake write+read can run concurrently.
type session struct {
	send cipher.AEAD
	recv cipher.AEAD
	// sendN / recvN are the next nonce counters (big-endian in 12-byte nonce).
	sendN  uint64
	recvN  uint64
	sendMu sync.Mutex
	recvMu sync.Mutex
}

// secureConn wraps a net.Conn with AES-GCM framed messages after ECDH.
// Wire for application frames: uint32be(len) || ciphertext
// where plaintext = type || payload, and ciphertext includes the GCM tag.
// Nonce = 4 zero bytes || uint64be(counter).
type secureConn struct {
	net.Conn
	sess *session
	// maxSize for plaintext type+payload
	maxSize int
}

func (c *secureConn) WriteFrame(typ uint8, payload []byte) error {
	c.sess.sendMu.Lock()
	defer c.sess.sendMu.Unlock()
	plain := make([]byte, 1+len(payload))
	plain[0] = typ
	copy(plain[1:], payload)
	if len(plain) > c.maxSize {
		return fmt.Errorf("p2p: encrypted message too large: %d > %d", len(plain), c.maxSize)
	}
	nonce := make([]byte, c.sess.send.NonceSize())
	binary.BigEndian.PutUint64(nonce[len(nonce)-8:], c.sess.sendN)
	c.sess.sendN++
	ct := c.sess.send.Seal(nil, nonce, plain, nil)
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(ct)))
	if _, err := c.Conn.Write(hdr[:]); err != nil {
		return err
	}
	_, err := c.Conn.Write(ct)
	return err
}

func (c *secureConn) ReadFrame() (Frame, error) {
	c.sess.recvMu.Lock()
	defer c.sess.recvMu.Unlock()
	var hdr [4]byte
	if _, err := io.ReadFull(c.Conn, hdr[:]); err != nil {
		return Frame{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	// ciphertext length: plaintext + GCM overhead
	overhead := c.sess.recv.Overhead()
	if n == 0 || int(n) > c.maxSize+overhead {
		return Frame{}, fmt.Errorf("p2p: encrypted frame length %d invalid", n)
	}
	ct := make([]byte, n)
	if _, err := io.ReadFull(c.Conn, ct); err != nil {
		return Frame{}, err
	}
	nonce := make([]byte, c.sess.recv.NonceSize())
	binary.BigEndian.PutUint64(nonce[len(nonce)-8:], c.sess.recvN)
	c.sess.recvN++
	plain, err := c.sess.recv.Open(nil, nonce, ct, nil)
	if err != nil {
		return Frame{}, fmt.Errorf("p2p: decrypt: %w", err)
	}
	if len(plain) < 1 {
		return Frame{}, fmt.Errorf("p2p: empty decrypted frame")
	}
	return Frame{Type: plain[0], Payload: plain[1:]}, nil
}

// performSecureHandshake runs mutual X25519 ECDH and returns a session.
// inbound: true if we accepted the TCP connection (server role for key mix).
func performSecureHandshake(conn net.Conn, chainID *big.Int, maxSize int, inbound bool) (*session, error) {
	curve := ecdh.X25519()
	eph, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	rnd := make([]byte, 32)
	if _, err := rand.Read(rnd); err != nil {
		return nil, err
	}
	local := secureHelloPayload{
		Version: SecureVersion,
		EphPub:  eph.PublicKey().Bytes(),
		Random:  rnd,
	}
	localEnc, err := rlp.EncodeToBytes([]interface{}{local.Version, local.EphPub, local.Random})
	if err != nil {
		return nil, err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- WriteFrame(conn, MsgSecureHello, localEnc, maxSize)
	}()
	frame, err := ReadFrame(conn, maxSize)
	if err != nil {
		return nil, fmt.Errorf("p2p: read secure hello: %w", err)
	}
	if werr := <-errCh; werr != nil {
		return nil, fmt.Errorf("p2p: write secure hello: %w", werr)
	}
	if frame.Type != MsgSecureHello {
		return nil, fmt.Errorf("p2p: expected secure hello (0x00), got 0x%02x — peer may be cleartext-only", frame.Type)
	}
	var raw struct {
		Version uint32
		EphPub  []byte
		Random  []byte
	}
	if err := rlp.DecodeBytes(frame.Payload, &raw); err != nil {
		return nil, fmt.Errorf("p2p: secure hello decode: %w", err)
	}
	if raw.Version != SecureVersion {
		return nil, fmt.Errorf("p2p: secure version mismatch: got %d", raw.Version)
	}
	if len(raw.EphPub) != 32 || len(raw.Random) != 32 {
		return nil, fmt.Errorf("p2p: secure hello key/random length")
	}
	remotePub, err := curve.NewPublicKey(raw.EphPub)
	if err != nil {
		return nil, fmt.Errorf("p2p: remote eph key: %w", err)
	}
	shared, err := eph.ECDH(remotePub)
	if err != nil {
		return nil, fmt.Errorf("p2p: ecdh: %w", err)
	}

	// Order randoms by inbound role so both sides agree on send/recv keys.
	// initiator (outbound) uses keys[0] for send; responder (inbound) uses keys[1] for send.
	var r1, r2 []byte
	if inbound {
		r1, r2 = raw.Random, local.Random // peer is initiator
	} else {
		r1, r2 = local.Random, raw.Random
	}
	cid := []byte{0}
	if chainID != nil {
		cid = chainID.Bytes()
	}
	// Derive 64 bytes: k_init_send || k_resp_send
	material := crypto.Keccak256(
		[]byte("Dew/Secure/1"),
		shared,
		r1,
		r2,
		cid,
	)
	material2 := crypto.Keccak256([]byte("Dew/Secure/2"), material)
	keyInit := material
	if len(keyInit) < 32 {
		return nil, fmt.Errorf("p2p: kdf short")
	}
	keyInit = keyInit[:32]
	keyResp := material2
	if len(keyResp) < 32 {
		return nil, fmt.Errorf("p2p: kdf short")
	}
	keyResp = keyResp[:32]

	aeadInit, err := newAESGCM(keyInit)
	if err != nil {
		return nil, err
	}
	aeadResp, err := newAESGCM(keyResp)
	if err != nil {
		return nil, err
	}

	sess := &session{}
	if inbound {
		// We are responder: send with keyResp, receive with keyInit
		sess.send = aeadResp
		sess.recv = aeadInit
	} else {
		sess.send = aeadInit
		sess.recv = aeadResp
	}
	return sess, nil
}

func newAESGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

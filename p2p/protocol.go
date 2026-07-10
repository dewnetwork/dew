package p2p

import (
	"fmt"
	"math/big"

	"github.com/dewnetwork/dew/core/types"
	"github.com/dewnetwork/dew/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// Message type IDs (docs/networking/p2p.md).
const (
	MsgHandshake    uint8 = 0x01
	MsgPing         uint8 = 0x02
	MsgPong         uint8 = 0x03 // wire reply to ping (doc groups ping/pong under 0x02; we split for clarity)
	MsgGetPeers     uint8 = 0x04
	MsgPeers        uint8 = 0x05
	MsgInventory    uint8 = 0x06
	MsgGetData      uint8 = 0x07
	MsgTxPayload    uint8 = 0x08
	MsgBlockPayload uint8 = 0x09
	MsgGetBlocks    uint8 = 0x0A
	MsgBlocks       uint8 = 0x0B

	// Consensus channel (Dew-BFT)
	MsgProposal  uint8 = 0x10
	MsgPrevote   uint8 = 0x11
	MsgPrecommit uint8 = 0x12
)

// Inventory kinds.
const (
	InvTx    uint8 = 1
	InvBlock uint8 = 2
)

// ProtocolVersion is the current handshake version.
const ProtocolVersion uint32 = 1

// DefaultMaxMsgSize is the maximum framed payload + header size (4 MiB).
const DefaultMaxMsgSize = 4 << 20

// PeerID is a 20-byte node identity (Ethereum-style address of the node key).
type PeerID = crypto.Address

// Handshake is exchanged on connect. Chain ID mismatch → disconnect.
type Handshake struct {
	Version    uint32
	ChainID    *big.Int
	Height     uint64
	HeadHash   types.Hash
	ListenPort uint16
	// NodeID is derived from the signing key (included for convenience).
	NodeID PeerID
	// Nonce is random 32 bytes; Signature covers SignBytes(Handshake).
	Nonce     []byte
	Signature []byte
}

// handshakeSignPayload is RLP material for the handshake signature.
type handshakeSignPayload struct {
	Domain     string
	Version    uint32
	ChainID    *big.Int
	Height     uint64
	HeadHash   []byte
	ListenPort uint16
	NodeID     []byte
	Nonce      []byte
}

const handshakeDomain = "Dew/Handshake/1"

// SignBytes returns the 32-byte digest for a handshake (signature field ignored).
func (h *Handshake) SignBytes() ([]byte, error) {
	enc, err := rlp.EncodeToBytes(&handshakeSignPayload{
		Domain:     handshakeDomain,
		Version:    h.Version,
		ChainID:    h.ChainID,
		Height:     h.Height,
		HeadHash:   h.HeadHash.Bytes(),
		ListenPort: h.ListenPort,
		NodeID:     h.NodeID.Bytes(),
		Nonce:      h.Nonce,
	})
	if err != nil {
		return nil, err
	}
	return crypto.Keccak256(enc), nil
}

// EncodeRLP encodes the handshake for the wire.
func (h *Handshake) Encode() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		h.Version,
		h.ChainID,
		h.Height,
		h.HeadHash.Bytes(),
		uint(h.ListenPort),
		h.NodeID.Bytes(),
		h.Nonce,
		h.Signature,
	})
}

// DecodeHandshake parses a handshake payload.
func DecodeHandshake(b []byte) (*Handshake, error) {
	var raw struct {
		Version    uint32
		ChainID    *big.Int
		Height     uint64
		HeadHash   []byte
		ListenPort uint
		NodeID     []byte
		Nonce      []byte
		Signature  []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, fmt.Errorf("p2p: handshake decode: %w", err)
	}
	h := &Handshake{
		Version:    raw.Version,
		ChainID:    raw.ChainID,
		Height:     raw.Height,
		HeadHash:   types.BytesToHash(raw.HeadHash),
		ListenPort: uint16(raw.ListenPort),
		Nonce:      raw.Nonce,
		Signature:  raw.Signature,
	}
	if len(raw.NodeID) != 20 {
		return nil, fmt.Errorf("p2p: handshake node id must be 20 bytes")
	}
	copy(h.NodeID[:], raw.NodeID)
	return h, nil
}

// InvItem is one inventory announcement entry.
type InvItem struct {
	Kind uint8
	Hash types.Hash
}

// Inventory message: list of available object hashes.
type Inventory struct {
	Items []InvItem
}

func (m *Inventory) Encode() ([]byte, error) {
	items := make([]interface{}, len(m.Items))
	for i, it := range m.Items {
		items[i] = []interface{}{it.Kind, it.Hash.Bytes()}
	}
	return rlp.EncodeToBytes(items)
}

func DecodeInventory(b []byte) (*Inventory, error) {
	var raw []struct {
		Kind uint
		Hash []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	m := &Inventory{Items: make([]InvItem, len(raw))}
	for i, r := range raw {
		m.Items[i] = InvItem{Kind: uint8(r.Kind), Hash: types.BytesToHash(r.Hash)}
	}
	return m, nil
}

// GetData requests full payloads by hash.
type GetData struct {
	Items []InvItem
}

func (m *GetData) Encode() ([]byte, error) {
	items := make([]interface{}, len(m.Items))
	for i, it := range m.Items {
		items[i] = []interface{}{it.Kind, it.Hash.Bytes()}
	}
	return rlp.EncodeToBytes(items)
}

func DecodeGetData(b []byte) (*GetData, error) {
	inv, err := DecodeInventory(b)
	if err != nil {
		return nil, err
	}
	return &GetData{Items: inv.Items}, nil
}

// TxPayload carries a raw transaction.
type TxPayload struct {
	Hash types.Hash
	Raw  []byte
}

func (m *TxPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{m.Hash.Bytes(), m.Raw})
}

func DecodeTxPayload(b []byte) (*TxPayload, error) {
	var raw struct {
		Hash []byte
		Raw  []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	return &TxPayload{Hash: types.BytesToHash(raw.Hash), Raw: raw.Raw}, nil
}

// BlockPayload carries an opaque block body (application-defined encoding).
type BlockPayload struct {
	Number uint64
	Hash   types.Hash
	Raw    []byte
}

func (m *BlockPayload) Encode() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{m.Number, m.Hash.Bytes(), m.Raw})
}

func DecodeBlockPayload(b []byte) (*BlockPayload, error) {
	var raw struct {
		Number uint64
		Hash   []byte
		Raw    []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	return &BlockPayload{Number: raw.Number, Hash: types.BytesToHash(raw.Hash), Raw: raw.Raw}, nil
}

// GetBlocks requests a contiguous height range [From, To] inclusive.
type GetBlocks struct {
	From uint64
	To   uint64
}

func (m *GetBlocks) Encode() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{m.From, m.To})
}

func DecodeGetBlocks(b []byte) (*GetBlocks, error) {
	var raw struct {
		From uint64
		To   uint64
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	return &GetBlocks{From: raw.From, To: raw.To}, nil
}

// BlocksMsg is a batch of blocks for sync.
type BlocksMsg struct {
	Blocks []BlockPayload
}

func (m *BlocksMsg) Encode() ([]byte, error) {
	list := make([]interface{}, len(m.Blocks))
	for i, bl := range m.Blocks {
		list[i] = []interface{}{bl.Number, bl.Hash.Bytes(), bl.Raw}
	}
	return rlp.EncodeToBytes(list)
}

func DecodeBlocksMsg(b []byte) (*BlocksMsg, error) {
	var raw []struct {
		Number uint64
		Hash   []byte
		Raw    []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	m := &BlocksMsg{Blocks: make([]BlockPayload, len(raw))}
	for i, r := range raw {
		m.Blocks[i] = BlockPayload{Number: r.Number, Hash: types.BytesToHash(r.Hash), Raw: r.Raw}
	}
	return m, nil
}

// PeerAddr is an advertised peer address for PEX.
type PeerAddr struct {
	ID   PeerID
	Host string
	Port uint16
}

// PeersMsg is a list of known peers.
type PeersMsg struct {
	Peers []PeerAddr
}

func (m *PeersMsg) Encode() ([]byte, error) {
	list := make([]interface{}, len(m.Peers))
	for i, p := range m.Peers {
		list[i] = []interface{}{p.ID.Bytes(), p.Host, uint(p.Port)}
	}
	return rlp.EncodeToBytes(list)
}

func DecodePeersMsg(b []byte) (*PeersMsg, error) {
	var raw []struct {
		ID   []byte
		Host string
		Port uint
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	m := &PeersMsg{Peers: make([]PeerAddr, len(raw))}
	for i, r := range raw {
		if len(r.ID) != 20 {
			return nil, fmt.Errorf("p2p: peer id must be 20 bytes")
		}
		var id PeerID
		copy(id[:], r.ID)
		m.Peers[i] = PeerAddr{ID: id, Host: r.Host, Port: uint16(r.Port)}
	}
	return m, nil
}

// WireProposal is a consensus proposal without the full block body optional fields
// encoded for the wire (hash + height/round + sig; optional block raw).
type WireProposal struct {
	Height    uint64
	Round     uint64
	BlockHash types.Hash
	Proposer  crypto.Address
	Signature []byte
	// BlockRaw is optional opaque block bytes (may be empty when hash-only).
	BlockRaw []byte
}

func (m *WireProposal) Encode() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		m.Height, m.Round, m.BlockHash.Bytes(), m.Proposer.Bytes(), m.Signature, m.BlockRaw,
	})
}

func DecodeWireProposal(b []byte) (*WireProposal, error) {
	var raw struct {
		Height    uint64
		Round     uint64
		BlockHash []byte
		Proposer  []byte
		Signature []byte
		BlockRaw  []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	m := &WireProposal{
		Height:    raw.Height,
		Round:     raw.Round,
		BlockHash: types.BytesToHash(raw.BlockHash),
		Signature: raw.Signature,
		BlockRaw:  raw.BlockRaw,
	}
	if len(raw.Proposer) != 20 {
		return nil, fmt.Errorf("p2p: proposal proposer must be 20 bytes")
	}
	copy(m.Proposer[:], raw.Proposer)
	return m, nil
}

// WireVote is a consensus prevote/precommit on the wire.
type WireVote struct {
	Type      uint8 // 1 prevote, 2 precommit
	Height    uint64
	Round     uint64
	BlockHash types.Hash
	Validator crypto.Address
	Signature []byte
}

func (m *WireVote) Encode() ([]byte, error) {
	return rlp.EncodeToBytes([]interface{}{
		uint(m.Type), m.Height, m.Round, m.BlockHash.Bytes(), m.Validator.Bytes(), m.Signature,
	})
}

func DecodeWireVote(b []byte) (*WireVote, error) {
	var raw struct {
		Type      uint
		Height    uint64
		Round     uint64
		BlockHash []byte
		Validator []byte
		Signature []byte
	}
	if err := rlp.DecodeBytes(b, &raw); err != nil {
		return nil, err
	}
	m := &WireVote{
		Type:      uint8(raw.Type),
		Height:    raw.Height,
		Round:     raw.Round,
		BlockHash: types.BytesToHash(raw.BlockHash),
		Signature: raw.Signature,
	}
	if len(raw.Validator) != 20 {
		return nil, fmt.Errorf("p2p: vote validator must be 20 bytes")
	}
	copy(m.Validator[:], raw.Validator)
	return m, nil
}

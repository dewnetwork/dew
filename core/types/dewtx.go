package types

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"

	"github.com/dewnetwork/dew/crypto"
	"github.com/dewnetwork/dew/params"
)

// DewTxType is the envelope tag for native transactions (not an EVM type byte).
// Wire format: 0xdf || RLP(signed payload) — 0xdf is outside Ethereum typed-tx range
// so accidental eth_sendRawTransaction decoding fails closed.
const DewTxType = byte(0xdf)

// DewTx is a Phase B native transaction with mandatory access list.
//
// Encoding (frozen for Phase B):
//
//	0xdf || RLP([
//	  version, chainId, nonce, sender, receiver, amount, fee, payload,
//	  accessList, yParity, r, s
//	])
//
// Signing hash = Keccak256(domain || RLP(unsigned fields)) where
// domain = Keccak256("DewTx:v1") — distinct from EVM tx hashes.
type DewTx struct {
	Version    uint32
	ChainID    *big.Int
	Nonce      uint64
	Sender     crypto.Address
	Receiver   crypto.Address
	Amount     *uint256.Int
	Fee        uint64 // flat fee in wei
	Payload    []byte
	AccessList []crypto.Address // declared accounts (read/write); mandatory
	V, R, S    *big.Int

	hash Hash
}

// NewDewTx builds an unsigned native transfer/call skeleton.
func NewDewTx(
	chainID *big.Int,
	nonce uint64,
	sender, receiver crypto.Address,
	amount *uint256.Int,
	fee uint64,
	payload []byte,
	accessList []crypto.Address,
) *DewTx {
	if amount == nil {
		amount = uint256.NewInt(0)
	}
	if fee == 0 {
		fee = params.DefaultDewTxFeeWei
	}
	al := make([]crypto.Address, len(accessList))
	copy(al, accessList)
	return &DewTx{
		Version:    params.DewTxVersion,
		ChainID:    new(big.Int).Set(chainID),
		Nonce:      nonce,
		Sender:     sender,
		Receiver:   receiver,
		Amount:     new(uint256.Int).Set(amount),
		Fee:        fee,
		Payload:    append([]byte(nil), payload...),
		AccessList: al,
		V:          new(big.Int),
		R:          new(big.Int),
		S:          new(big.Int),
	}
}

type dewTxUnsignedRLP struct {
	Version    uint32
	ChainID    *big.Int
	Nonce      uint64
	Sender     []byte
	Receiver   []byte
	Amount     *big.Int
	Fee        uint64
	Payload    []byte
	AccessList [][]byte
}

type dewTxSignedRLP struct {
	Version    uint32
	ChainID    *big.Int
	Nonce      uint64
	Sender     []byte
	Receiver   []byte
	Amount     *big.Int
	Fee        uint64
	Payload    []byte
	AccessList [][]byte
	YParity    *big.Int
	R, S       *big.Int
}

func (tx *DewTx) unsignedRLP() dewTxUnsignedRLP {
	al := make([][]byte, len(tx.AccessList))
	for i, a := range tx.AccessList {
		al[i] = a.Bytes()
	}
	amt := big.NewInt(0)
	if tx.Amount != nil {
		amt = tx.Amount.ToBig()
	}
	return dewTxUnsignedRLP{
		Version:    tx.Version,
		ChainID:    copyBig(tx.ChainID),
		Nonce:      tx.Nonce,
		Sender:     tx.Sender.Bytes(),
		Receiver:   tx.Receiver.Bytes(),
		Amount:     amt,
		Fee:        tx.Fee,
		Payload:    tx.Payload,
		AccessList: al,
	}
}

func (tx *DewTx) signedRLP() dewTxSignedRLP {
	u := tx.unsignedRLP()
	return dewTxSignedRLP{
		Version:    u.Version,
		ChainID:    u.ChainID,
		Nonce:      u.Nonce,
		Sender:     u.Sender,
		Receiver:   u.Receiver,
		Amount:     u.Amount,
		Fee:        u.Fee,
		Payload:    u.Payload,
		AccessList: u.AccessList,
		YParity:    copyBig(tx.V),
		R:          copyBig(tx.R),
		S:          copyBig(tx.S),
	}
}

// SigningHash returns the domain-separated digest to sign.
func (tx *DewTx) SigningHash() Hash {
	enc, err := rlp.EncodeToBytes(tx.unsignedRLP())
	if err != nil {
		panic("types: dewtx unsigned encode: " + err.Error())
	}
	domain := crypto.Keccak256([]byte(params.DewTxDomainTag))
	return Keccak256Hash(domain, enc)
}

// Hash returns Keccak-256 of the signed wire encoding.
func (tx *DewTx) Hash() Hash {
	if tx.hash != (Hash{}) {
		return tx.hash
	}
	enc, err := tx.MarshalBinary()
	if err != nil {
		panic("types: dewtx encode: " + err.Error())
	}
	tx.hash = Keccak256Hash(enc)
	return tx.hash
}

// MarshalBinary returns 0xdf || RLP(signed payload).
func (tx *DewTx) MarshalBinary() ([]byte, error) {
	payload, err := rlp.EncodeToBytes(tx.signedRLP())
	if err != nil {
		return nil, err
	}
	return append([]byte{DewTxType}, payload...), nil
}

// UnmarshalBinary decodes a signed DewTx envelope.
func (tx *DewTx) UnmarshalBinary(b []byte) error {
	if len(b) < 2 || b[0] != DewTxType {
		return fmt.Errorf("types: not a DewTx (missing 0xdf prefix)")
	}
	var raw dewTxSignedRLP
	if err := rlp.DecodeBytes(b[1:], &raw); err != nil {
		return fmt.Errorf("types: dewtx rlp: %w", err)
	}
	if raw.Version != params.DewTxVersion {
		return fmt.Errorf("types: unsupported DewTx version %d", raw.Version)
	}
	if len(raw.Sender) != 20 || len(raw.Receiver) != 20 {
		return fmt.Errorf("types: dewtx address length")
	}
	var sender, receiver crypto.Address
	copy(sender[:], raw.Sender)
	copy(receiver[:], raw.Receiver)
	al := make([]crypto.Address, len(raw.AccessList))
	for i, a := range raw.AccessList {
		if len(a) != 20 {
			return fmt.Errorf("types: dewtx access list address length")
		}
		copy(al[i][:], a)
	}
	amt := uint256.NewInt(0)
	if raw.Amount != nil {
		if overflow := amt.SetFromBig(raw.Amount); overflow {
			return fmt.Errorf("types: dewtx amount overflow")
		}
	}
	tx.Version = raw.Version
	tx.ChainID = copyBig(raw.ChainID)
	tx.Nonce = raw.Nonce
	tx.Sender = sender
	tx.Receiver = receiver
	tx.Amount = amt
	tx.Fee = raw.Fee
	tx.Payload = append([]byte(nil), raw.Payload...)
	tx.AccessList = al
	tx.V = copyBig(raw.YParity)
	tx.R = copyBig(raw.R)
	tx.S = copyBig(raw.S)
	tx.hash = Hash{}
	return nil
}

// SignDewTx signs tx with key and fills V,R,S. Requires key address == tx.Sender.
func SignDewTx(tx *DewTx, key *ecdsa.PrivateKey) error {
	sig, err := crypto.Sign(tx.SigningHash().Bytes(), key)
	if err != nil {
		return err
	}
	tx.R = new(big.Int).SetBytes(sig[0:32])
	tx.S = new(big.Int).SetBytes(sig[32:64])
	tx.V = new(big.Int).SetUint64(uint64(sig[64]))
	tx.hash = Hash{}
	addr := crypto.PubkeyToAddress(&key.PublicKey)
	if addr != tx.Sender {
		return fmt.Errorf("types: dewtx sender mismatch: key=%s tx=%s", addr.Hex(), tx.Sender.Hex())
	}
	return nil
}

// RecoverSender recovers the signer address from V,R,S and checks it equals Sender.
func (tx *DewTx) RecoverSender() (crypto.Address, error) {
	if tx.R == nil || tx.S == nil || tx.V == nil {
		return crypto.Address{}, fmt.Errorf("types: dewtx missing signature")
	}
	sig := make([]byte, 65)
	rb := tx.R.Bytes()
	sb := tx.S.Bytes()
	copy(sig[32-len(rb):32], rb)
	copy(sig[64-len(sb):64], sb)
	v := tx.V.Uint64()
	if v > 1 {
		return crypto.Address{}, fmt.Errorf("types: dewtx invalid yParity %d", v)
	}
	sig[64] = byte(v)
	pub, err := crypto.Ecrecover(tx.SigningHash().Bytes(), sig)
	if err != nil {
		return crypto.Address{}, fmt.Errorf("types: dewtx ecrecover: %w", err)
	}
	if len(pub) != 65 {
		return crypto.Address{}, fmt.Errorf("types: dewtx bad pubkey len")
	}
	var addr crypto.Address
	h := crypto.Keccak256(pub[1:])
	copy(addr[:], h[12:])
	if addr != tx.Sender {
		return crypto.Address{}, fmt.Errorf("types: dewtx signature does not match sender")
	}
	return addr, nil
}

// ContainsAccess reports whether addr is sender, receiver, or in AccessList.
func (tx *DewTx) ContainsAccess(addr crypto.Address) bool {
	if addr == tx.Sender || addr == tx.Receiver {
		return true
	}
	for _, a := range tx.AccessList {
		if a == addr {
			return true
		}
	}
	return false
}

// Package p2p implements Dew networking (Phase A6 + C2 + D3b): TCP transport, framed
// messages, optional encrypted sessions, handshake, durable peer store + auto-redial,
// inventory gossip for txs/blocks, catch-up sync, and delivery of Dew-BFT consensus messages.
//
// Wire framing (docs/networking/p2p.md):
//
//	Cleartext: uint32be length || uint8 type || payload
//	Encrypted (default): MsgSecureHello exchange, then
//	  uint32be length || AES-256-GCM(ciphertext of type||payload)
//
// Cipher suite: X25519 ECDH, keys via Keccak domain "Dew/Secure/1|2", AES-256-GCM.
// Identity handshake (chain ID + node key signature) runs inside the encrypted
// channel. Set Encrypt=false and AllowCleartext=true for explicit loopback dev only.
package p2p

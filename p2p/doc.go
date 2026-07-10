// Package p2p implements Dew networking (Phase A6): TCP transport, framed
// messages, handshake, peer store, inventory gossip for txs/blocks, catch-up
// sync, and delivery of Dew-BFT consensus messages.
//
// Wire framing (docs/networking/p2p.md):
//
//	uint32be length || uint8 type || payload
//
// Payloads use RLP. Transport is cleartext TCP suitable for private devnets;
// encrypted transport is deferred until public-network hardening.
package p2p

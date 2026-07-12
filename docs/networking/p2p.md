---
title: P2P Layer
description: Transport, node identity, discovery, and message framing.
category: networking
order: 10
status: draft
---

# P2P Layer

## Transport

| Item          | Spec                                             |
| :------------ | :----------------------------------------------- |
| Primary       | TCP                                              |
| Optional      | WebSocket (light / browser-oriented peers later) |
| Default port  | `30303` (configurable)                           |
| Framing       | See cleartext vs encrypted below                 |
| Payload codec | RLP (current); Protocol Buffers later (_tentative_) |

### Cleartext framing (dev only)

```
uint32be length || uint8 type || payload
```

`length` counts `type + payload`. Enabled only when `Encrypt=false` **and** `AllowCleartext=true`.

### Encrypted transport (Phase C2 — default)

1. **Secure hello** (still cleartext frames of type `0x00`): each side sends X25519 ephemeral pubkey + 32-byte random.
2. **ECDH** shared secret → two AES-256-GCM keys via Keccak domain tags `Dew/Secure/1` and `Dew/Secure/2` (initiator send key vs responder send key), mixed with both randoms and chain ID.
3. **Application frames** (including identity handshake):  
   `uint32be length || AES-256-GCM(ciphertext)` where plaintext is `type || payload`.  
   Nonce = 12 bytes with a per-direction uint64 counter in the last 8 bytes.

Identity handshake (chain ID + node key signature) runs **inside** the encrypted session. Cleartext is not acceptable on public or multi-host private nets.

| Go config | Meaning |
| :-------- | :------ |
| `Encrypt: true` (default) | Require secure hello + GCM |
| `Encrypt: false` + `AllowCleartext: true` | Explicit dev cleartext |

## Node identity

```
dew://<hex_public_key>@<host>:<port>
```

Handshake authenticates the peer’s key after the encrypted session is established (or on cleartext when explicitly allowed).

## Discovery

1. **Bootstrap seeds** from config (`--p2p.bootnodes`)
2. **PEX**: `GetPeers` / `Peers` exchange (up to 256 addresses)
3. **Durable peer store (D3b):** `<datadir>/peers.json` when `--datadir` is set
4. **Auto-redial:** background maintain loop redials known + bootnode addrs after disconnect (backoff 1s…5 min; 7d TTL eviction; ban score ≥ 100 skips dial)
5. Maintain target **~25** active peers (_tentative_)

```
Bootstrap → Handshake → GetPeers → Dial more → Maintain peer count
         ↘ load peers.json ↗          ↘ on disconnect: schedule redial
```

## Message type IDs (_draft allocation_)

| ID            | Name                             | Category      |
| :------------ | :------------------------------- | :------------ |
| `0x00`        | SecureHello (X25519)             | Session (C2)  |
| `0x01`        | Handshake                        | Session       |
| `0x02`        | Ping / Pong                      | Session       |
| `0x03`        | GetPeers                         | Discovery     |
| `0x04`        | Peers                            | Discovery     |
| `0x05`        | Inventory                        | Gossip        |
| `0x06`        | GetData                          | Gossip / Sync |
| `0x07`        | TxPayload                        | Gossip        |
| `0x08`        | BlockPayload                     | Gossip / Sync |
| `0x10`–`0x1F` | Consensus (Proposal, Prevote, …) | Dew-BFT       |
| `0x20`        | Evidence                         | Security      |

## Handshake contents

- Protocol version
- Chain ID
- Best height + app/block hash
- Node role flags (optional)

Mismatch on chain ID → disconnect.

## DoS basics

- Max message size
- Rate limits per peer
- Ban scoring for invalid blocks/txs
- Separate queues for consensus vs bulk sync (avoid starving votes)

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
| Framing       | `uint32be length \|\| uint8 type \|\| payload`   |
| Payload codec | Protocol Buffers (_tentative_)                   |

## Node identity

```
dew://<hex_public_key>@<host>:<port>
```

Handshake authenticates the peer’s key. Long-term: authenticated encryption (e.g. Noise or TLS) — Phase A may start with cleartext **only on private devnets**; public networks need encrypted transport before advertising security.

## Discovery

1. **Bootstrap seeds** from config
2. **PEX**: `GetPeers` / `Peers` exchange (up to 256 addresses)
3. Maintain target **~25** active peers (_tentative_)

```
Bootstrap → Handshake → GetPeers → Dial more → Maintain peer count
```

## Message type IDs (_draft allocation_)

| ID            | Name                             | Category      |
| :------------ | :------------------------------- | :------------ |
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

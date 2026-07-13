---
title: Try public testnet (5 minutes)
description: Browser-only path — faucet, MetaMask, Guestbook sign, explorer on public-testnet-v1.
category: ops
order: 34
status: stable
---

# Try public testnet (5 minutes)

Touch **Dew public-testnet-v1** without building a node or installing Foundry. Chain ID is always **`2205`**.

| Step | Action | URL / value |
| :--- | :--- | :--- |
| 1 | Request test DEW | [Faucet](https://faucet-dew.fadosoft.com) (captcha · 1 DEW / address / 24h) |
| 2 | Add network in MetaMask | RPC `https://rpc-dew.fadosoft.com` · chain ID **2205** · symbol **DEW** |
| 3 | Sign the Guestbook | [Guestbook SPA](https://guestbook-dew.fadosoft.com) |
| 4 | Open explorer | [Explorer](https://explorer-dew.fadosoft.com) — tx / address deep-links from the SPA |
| 5 | (Optional) Deploy Solidity | [Quick start](./quickstart.md) · [Recipes](./recipes.md) |

Freeze tag: **`public-testnet-v1`**. Ops surface: [Public testnet freeze](./public-testnet.md). This is **path B** (single-host controlled RPC) — not mainnet and not multi-host Path A.

---

## 1. Faucet

1. Open `https://faucet-dew.fadosoft.com`.  
2. Paste a **new** wallet address (never use Anvil / Hardhat default keys on public nets).  
3. Complete captcha → receive **1 DEW** (rate limits: 1 / address / 24h · 10 / IP / hour).

Policy details: [Production faucet](../product/faucet.md) · [Public testnet freeze](./public-testnet.md#faucet-policy).

---

## 2. MetaMask network

**One-click (EIP-3085):** on the [landing site](https://dewnetwork.github.io/dew/) use **Add to wallet**, or connect in the [Guestbook SPA](https://guestbook-dew.fadosoft.com) — both call `wallet_switchEthereumChain` / `wallet_addEthereumChain` for chain **2205**.

**Manual — Settings → Networks → Add network:**

| Field | Value |
| :--- | :--- |
| Network name | Dew public-testnet-v1 |
| RPC URL | `https://rpc-dew.fadosoft.com` |
| Chain ID | `2205` |
| Currency symbol | DEW |
| Block explorer URL | `https://explorer-dew.fadosoft.com` (**base only**, no path suffix) |

Confirm balance shows the faucet drip.

---

## 3. Guestbook — read and sign

1. Open `https://guestbook-dew.fadosoft.com`.  
2. **Connect** wallet (switch / add network if prompted).  
3. Enter a short message → **Sign**.  
4. Wait for the receipt; use the SPA link to the transaction on the explorer.

| Item | Value |
| :--- | :--- |
| SPA | `https://guestbook-dew.fadosoft.com` |
| Contract | `0x83bB4E539BE46503481E66094b01b854990BF84a` |

Read-only works without a wallet (**Refresh**). Product notes: [Guestbook demo](../product/guestbook.md).

---

## 4. Explorer

| Kind | Example shape |
| :--- | :--- |
| Transaction | `https://explorer-dew.fadosoft.com/tx/<hash>` |
| Address / contract | `https://explorer-dew.fadosoft.com/address/<addr>` |

Home: `https://explorer-dew.fadosoft.com`. Routes: [Block explorer](../product/block-explorer.md).

Optional MetaMask custom tokens (mock / test only — live path B):

| Symbol | Decimals | Address |
| :--- | ---: | :--- |
| USDT | 6 | `0x43d08b71B0A5626a9D0eB607C2aE5277255cFbC8` |
| USDC | 6 | `0xacDd0BF26C96879b4c4CeE8F92928f1760F07119` |
| DAI | 18 | `0x9813B1738eb2982F3D0C1E22F9C7c4F07B670a1B` |
| WETH | 18 | `0x5b88b2ab38920197328f9D90BaAf29cb91F7E3CB` |
| WBTC | 8 | `0xdB6E09cb954Ae645C815Bc941bDfD85D4144166E` |

Full table: [public-testnet.md](./public-testnet.md#live-mock-erc-20-basket-path-b).

Optional smoke from a machine with Node:

```bash
node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com
# expect eth_chainId 0x89d (2205)
```

---

## 5. Next (optional) — ship code

| Goal | Doc |
| :--- | :--- |
| Local `dew devnet` + Foundry ERC-20 | [Quick start (5 minutes)](./quickstart.md) |
| Guestbook / multi-tx scripts | [Builder recipes](./recipes.md) |
| Foundry sample project | [examples/foundry](../../examples/foundry/) |

Public deploy uses the **same** chain ID and a faucet-funded key (never Anvil #0).

---

## Safety

| Do | Don't |
| :--- | :--- |
| New wallet for public testnet | Reuse Anvil #0 / tutorial keys on public RPC |
| Treat DEW here as **test** gas only | Expect mainnet value or uptime SLAs |
| Stop at the SPA if you only need a demo | Expose validator or faucet keys in the browser |

Path A (multi-host validators + bootnodes) is **not** required for this flow — see [D3 scale](../scale/d3-scale.md) when operators want multi-host public.

---

## Related

- [Public testnet freeze](./public-testnet.md)
- [Launch checklist](./launch-checklist.md) (publish template)
- [Guestbook (product)](../product/guestbook.md)
- [Quick start](./quickstart.md)
- [Builder recipes](./recipes.md)

# Deploy packaging

Operator samples for **private staging** and multi-process layout after Phase C freeze.

| Artifact | Role |
| :--- | :--- |
| [Dockerfile](./Dockerfile) | Multi-stage build of `dew` |
| [docker-compose.yml](./docker-compose.yml) | Devnet soak + optional 3-node mesh |
| [systemd/dew.service](./systemd/dew.service) | Single-host RPC unit |
| [Launch checklist](../docs/development/launch-checklist.md) | End-to-end ops checklist |

## Prerequisites

- Docker Engine + Compose v2 (for containers)
- Or Go 1.23+ for host binary builds
- Node only for smoke scripts (`scripts/smoke-rpc.mjs`)

## Quick private soak (recommended)

From **repo root** (`deploy/.env` sets `COMPOSE_PROFILES=devnet`):

```bash
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build
# equivalent:
docker compose -f deploy/docker-compose.yml --profile devnet up --build
```

- RPC: `http://127.0.0.1:8545`
- Smoke: `node scripts/smoke-rpc.mjs http://127.0.0.1:8545`
- ERC-20: `node scripts/devnet-erc20.mjs http://127.0.0.1:8545`

This runs **`dew devnet`** in one container (3 BFT validators + encrypted loopback P2P + RPC). Best path for 24–48h staging.

Without Docker:

```bash
go build -o bin/dew ./cmd/dew
./bin/dew devnet --http.addr 0.0.0.0 --http.port 8545
```

## Multi-process mesh (profile `multi`)

Do **not** combine with `devnet` (both want host `:8545`).

```bash
# Stop any prior stack first
docker compose -f deploy/docker-compose.yml --profile devnet down
docker compose -f deploy/docker-compose.yml --profile multi up --build
```

| Container | JSON-RPC (host) | P2P (host) |
| :--- | :--- | :--- |
| `dew-node-0` | `:8545` | `:30303` |
| `dew-node-1` | `:8546` | `:30304` |
| `dew-node-2` | `:8547` | `:30305` |

Each process:

- Loads the same genesis (`chainId` 2026)
- Listens for encrypted P2P and dials the other containers
- Serves JSON-RPC with **dev auto-mine** per accepted tx

**Limits (honest):** multi-process **Dew-BFT shared block production** is not fully wired yet; each node seals its own chain from the shared genesis. Use `multi` for packaging, ports, bootnode wiring, and encrypted transport practice. Use default `devnet` for consensus + RPC application smoke.

Compose uses **Anvil #0–#2** keys for private packaging only. **Never** reuse on a public net.

## systemd

See [systemd/dew.service](./systemd/dew.service). Create user/dirs:

```bash
sudo useradd --system --home /var/lib/dew --shell /usr/sbin/nologin dew || true
sudo mkdir -p /var/lib/dew /etc/dew
sudo install -m 755 bin/dew /usr/local/bin/dew
sudo install -m 644 genesis.json /etc/dew/genesis.json
sudo chown -R dew:dew /var/lib/dew
sudo install -m 644 deploy/systemd/dew.service /etc/systemd/system/dew.service
sudo systemctl daemon-reload
sudo systemctl enable --now dew
```

## Related residual

- Persistent peer store / auto-redial after restart — `agents/debt.md` (C5)
- Production faucet, public bootnode hostnames — C6 ops residual

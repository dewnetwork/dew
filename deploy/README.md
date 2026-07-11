# Deploy packaging

Operator samples for **private soak** and **controlled public RPC** after Phase C freeze.

| Artifact | Role |
| :--- | :--- |
| [Dockerfile](./Dockerfile) | Multi-stage build of `dew` |
| [docker-compose.yml](./docker-compose.yml) | **Public path B** — Dew + nginx (no open `:8545`) |
| [docker-compose.soak.yml](./docker-compose.soak.yml) | Private soak — devnet + optional 3-node mesh |
| [soak.env.example](./soak.env.example) | Sample soak env (`cp` → `soak.env`, gitignored) |
| [public.env.example](./public.env.example) | Sample public env (`cp` → `public.env`, gitignored) |
| [nginx/dew-rpc.conf](./nginx/dew-rpc.conf) | Host/systemd TLS reverse proxy sample |
| [nginx/dew-rpc.docker.conf](./nginx/dew-rpc.docker.conf) | Compose public proxy (upstream `dew:8545`) |
| [systemd/dew.service](./systemd/dew.service) | Single-host RPC unit (general) |
| [systemd/dew-rpc-public.service](./systemd/dew-rpc-public.service) | Public path B: loopback-only RPC (no Docker) |
| [public-rpc-single-host.md](./public-rpc-single-host.md) | Public runbook (Compose **or** systemd) |
| [Launch checklist](../docs/development/launch-checklist.md) | End-to-end ops checklist |

## Prerequisites

- Docker Engine + Compose v2 (for containers)
- Or Go 1.23+ for host binary builds
- Node only for smoke scripts (`scripts/smoke-rpc.mjs`)

## Quick private soak

From **repo root**:

```bash
cp deploy/soak.env.example deploy/soak.env   # once; edit ports/profile as needed
docker compose -f deploy/docker-compose.soak.yml --env-file deploy/soak.env up --build
# equivalent:
docker compose -f deploy/docker-compose.soak.yml --profile devnet up --build
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

## Multi-process mesh (soak profile `multi`)

Do **not** combine with `devnet` (both want host `:8545`).

```bash
# Stop any prior soak stack first
docker compose -f deploy/docker-compose.soak.yml --profile devnet down
docker compose -f deploy/docker-compose.soak.yml --profile multi up --build
```

| Container | JSON-RPC (host) | P2P (host) |
| :--- | :--- | :--- |
| `dew-node-0` | `:8545` | `:30303` |
| `dew-node-1` | `:8546` | `:30304` |
| `dew-node-2` | `:8547` | `:30305` |

Each process:

- Loads the same genesis (`chainId` 2205)
- Listens for encrypted P2P and dials the other containers
- Serves JSON-RPC with **dev auto-mine** per accepted tx

**Limits (honest):** multi-process **Dew-BFT shared block production** is not fully wired yet; each node seals its own chain from the shared genesis. Use `multi` for packaging, ports, bootnode wiring, and encrypted transport practice. Use default soak `devnet` for consensus + RPC application smoke.

Compose uses **Anvil #0–#2** keys for private packaging only. **Never** reuse on a public net.

## Controlled public RPC (Compose — path B)

After private soak. Dew is **not** published on host `:8545`; only nginx is on `:80` / `:443`.

```bash
# Stop soak first if it holds ports 80/443 (usually not) or 8545
docker compose -f deploy/docker-compose.soak.yml --profile devnet down

cp deploy/public.env.example deploy/public.env   # once
docker compose -f deploy/docker-compose.yml --env-file deploy/public.env up --build
```

| Service | Role | Host ports |
| :--- | :--- | :--- |
| `dew-rpc` | `dew run` on internal network only | none |
| `dew-rpc-proxy` | nginx rate-limit + proxy → `dew:8545` | `:80`, `:443` |

Smoke (HTTP until TLS is configured):

```bash
node scripts/smoke-rpc.mjs http://127.0.0.1
# or: curl -s -X POST http://127.0.0.1 -H 'content-type: application/json' \
#   -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

### TLS with Compose

1. Obtain certs (certbot on host, or ACME elsewhere).
2. Place `fullchain.pem` + `privkey.pem` under `deploy/certs/`.
3. Uncomment the `./certs` volume on `proxy` in `docker-compose.yml`.
4. Uncomment the HTTPS `server` block in `nginx/dew-rpc.docker.conf`.
5. `docker compose -f deploy/docker-compose.yml up -d --force-recreate proxy`

Firewall: allow 22/80/443 only — **never** publish container `:8545` on `0.0.0.0`.

### Systemd alternative (no Docker)

See [public-rpc-single-host.md](./public-rpc-single-host.md) — Dew on `127.0.0.1:8545`, host nginx on `:443`.

## systemd (binary install)

See [systemd/dew.service](./systemd/dew.service) (general) or [dew-rpc-public.service](./systemd/dew-rpc-public.service) (path B). Create user/dirs:

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
- Production faucet — D2: `cmd/dewfaucet`, [docs/development/faucet.md](../docs/development/faucet.md), [faucet.env.example](./faucet.env.example), [systemd/dewfaucet.service](./systemd/dewfaucet.service)
- Public bootnode hostnames — C6 ops residual

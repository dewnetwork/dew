# Deploy packaging

Operator samples for **private soak** and **controlled public RPC** after Phase C freeze.

| Directory / Artifact | Role |
| :--- | :--- |
| **[node/](./node/)** | **Core Node Deployment** |
| ├── [Dockerfile](./node/Dockerfile) | Multi-stage build of `dew` |
| ├── [docker-compose.yml](./node/docker-compose.yml) | **Public path B** — Dew + nginx (no open `:8545`) |
| ├── [docker-compose.soak.yml](./node/docker-compose.soak.yml) | Private soak — devnet + optional 3-node mesh |
| ├── [soak.env.example](./node/soak.env.example) | Sample soak env (`cp` → `soak.env`, gitignored) |
| ├── [public.env.example](./node/public.env.example) | Sample public env (`cp` → `public.env`, gitignored) |
| ├── [nginx/](./node/nginx/) | Nginx proxy configurations |
| ├── [systemd/](./node/systemd/) | Systemd service files for Node |
| └── [public-rpc-single-host.md](./node/public-rpc-single-host.md) | Public runbook (Compose **or** systemd) |
| **[faucet/](./faucet/)** | **Faucet Service Deployment** |
| ├── [Dockerfile](./faucet/Dockerfile) | Go faucet backend container |
| ├── [Dockerfile.web](./faucet/Dockerfile.web) | React faucet frontend container |
| ├── [docker-compose.yml](./faucet/docker-compose.yml) | Orchestrate backend & frontend |
| ├── [faucet.env.example](./faucet/faucet.env.example) | Faucet environment variable config |
| └── [systemd/](./faucet/systemd/) | Systemd service files for Faucet |
| **[explorer/](./explorer/)** | **Block Explorer SPA Deployment** |
| ├── [Dockerfile](./explorer/Dockerfile) | Multi-stage Vite build + nginx |
| ├── [docker-compose.yml](./explorer/docker-compose.yml) | Standalone explorer container |
| ├── [explorer.env.example](./explorer/explorer.env.example) | `PUBLIC_RPC_URL` / chain / base URL |
| └── [nginx/](./explorer/nginx/) | SPA `try_files` for client routes |
| **[nginx/](./nginx/)** | **Edge TLS configs** |
| ├── [dew-edge.docker.conf](./nginx/dew-edge.docker.conf) | **Compose edge** — rpc/faucet/explorer by Host |
| ├── [docker-entrypoint-edge.sh](./nginx/docker-entrypoint-edge.sh) | LE / optional origin PEMs / bootstrap + reload |
| ├── [certbot-entrypoint.sh](./nginx/certbot-entrypoint.sh) | webroot or DNS-01 Cloudflare + renew |
| └── [dew-edge.conf](./nginx/dew-edge.conf) | Host nginx alternative (loopback backends) |
| **[scripts/](./scripts/)** | **Certbot helpers** |
| ├── [setup-certbot-docker.sh](./scripts/setup-certbot-docker.sh) | **Optional** force LE issue + reload edge |
| ├── [install-edge-nginx.sh](./scripts/install-edge-nginx.sh) | Host nginx site install |
| └── [setup-certbot.sh](./scripts/setup-certbot.sh) | Host certbot (no Compose edge) |
| [docker-compose.yml](./docker-compose.yml) | **Full stack** — node + faucet + explorer + edge + certbot |
| [Launch checklist](../docs/ops/launch-checklist.md) | End-to-end ops checklist |

## Prerequisites

- Docker Engine + Compose v2 (for containers)
- Or Go 1.23+ for host binary builds
- Node only for smoke scripts (`scripts/smoke-rpc.mjs`)

## Quick private soak

From **repo root**:

```bash
cp deploy/node/soak.env.example deploy/node/soak.env   # once; edit ports/profile as needed
docker compose -f deploy/node/docker-compose.soak.yml --env-file deploy/node/soak.env up --build
# equivalent:
docker compose -f deploy/node/docker-compose.soak.yml --profile devnet up --build
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
docker compose -f deploy/node/docker-compose.soak.yml --profile devnet down
docker compose -f deploy/node/docker-compose.soak.yml --profile multi up --build
```

| Container | Role | JSON-RPC (host) | P2P (host) |
| :--- | :--- | :--- | :--- |
| `dew-node-0` | BFT validator (`--validator`) | `:8545` | `:30303` |
| `dew-node-1` | BFT validator | `:8546` | `:30304` |
| `dew-node-2` | BFT validator | `:8547` | `:30305` |
| `dew-node-rpc` | Full node (`--no-auto-mine`) | `:8548` | (internal) |

Each process:

- Loads the same genesis (`chainId` 2205)
- Listens for encrypted P2P and dials bootnodes (D3b: `peers.json` under `/var/lib/dew`)
- Validators share one canonical chain via Dew-BFT (D3a); RPC node syncs commits (no local seal)

Smoke after `up`:

```bash
node scripts/smoke-rpc.mjs http://127.0.0.1:8548
node scripts/devnet-erc20.mjs http://127.0.0.1:8548
```

Optional pace: `--bft.min-block-interval 1s` (default when unset). Use in-process soak profile `devnet` for the lightest 24–48h staging path. Spec: [docs/scale/d3-scale.md](../docs/scale/d3-scale.md).

Compose uses **Anvil #0–#2** keys for private packaging only. **Never** reuse on a public net.

## Controlled public RPC (Compose — path B)

After private soak. Dew is **not** published on host `:8545`; only nginx is on `:80` / `:443`.

```bash
# Stop soak first if it holds ports 80/443 (usually not) or 8545
docker compose -f deploy/node/docker-compose.soak.yml --profile devnet down

cp deploy/node/public.env.example deploy/node/public.env   # once
docker compose -f deploy/node/docker-compose.yml --env-file deploy/node/public.env up --build
```

| Service | Role | Host ports |
| :--- | :--- | :--- |
| `dew-rpc` | `dew run` on internal network only | none |
| `dew-rpc-proxy` | nginx rate-limit + proxy → `dew:8545` | `:80`, `:443` |

Smoke (HTTP before TLS):

```bash
node scripts/smoke-rpc.mjs http://127.0.0.1
# expect chainId 2205 / 0x89d
```

### TLS with Compose (in-container certs)

Prefer **host edge + certbot** (next section) for Let's Encrypt auto-renew. In-container TLS:

1. Obtain certs (certbot on host, or ACME elsewhere).
2. Place `fullchain.pem` + `privkey.pem` under `deploy/node/certs/`.
3. Uncomment the `./certs` volume on `proxy` in `docker-compose.yml`.
4. Uncomment the HTTPS `server` block in `nginx/dew-rpc.docker.conf`.
5. `docker compose -f deploy/node/docker-compose.yml up -d --force-recreate proxy`

Firewall: allow 22/80/443 only — **never** publish container `:8545` on `0.0.0.0`.

### Systemd alternative (no Docker)

See [public-rpc-single-host.md](./node/public-rpc-single-host.md) — Dew on `127.0.0.1:8545`, host nginx on `:443`.

## Public edge: nginx :443 + certbot (`*-dew.fadosoft.com`)

### Recommended: Docker Compose edge

Full stack in one compose file: backends **internal** + `edge` on **:80/:443** + `certbot` **auto-issue + renew**.

| Hostname | Upstream (compose) | Role |
| :--- | :--- | :--- |
| `rpc-dew.fadosoft.com` | `dew-node:8545` | JSON-RPC (rate limit, POST/OPTIONS) |
| `faucet-dew.fadosoft.com` | `faucet-frontend:80` | Faucet SPA + `/api` |
| `explorer-dew.fadosoft.com` | `explorer:80` | Block explorer SPA |

```bash
# 1) DNS A/AAAA: rpc / faucet / explorer → this host (orange-cloud OK with DNS-01)

# 2) Firewall: 22, 80, 443 only (not 8545)

# 3) .env — TLS via env (no certs/ directory)
cp deploy/.env.example deploy/.env
# DNS-01 (recommended behind Cloudflare orange-cloud):
#   CERTBOT_AUTH=dns-cloudflare
#   CERTBOT_IMAGE=certbot/dns-cloudflare:v2.11.0
#   CERTBOT_EMAIL=ops@…
#   CLOUDFLARE_API_TOKEN=…   # Zone → DNS → Edit

docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build -d
docker compose -f deploy/docker-compose.yml logs -f certbot edge

# 4) Verify
curl -sI https://faucet-dew.fadosoft.com | head -5
node scripts/smoke-rpc.mjs https://rpc-dew.fadosoft.com
```

| Service | Role | Host ports |
| :--- | :--- | :--- |
| `dew-node` | Devnet JSON-RPC | none |
| `faucet-backend` | Faucet API | none |
| `faucet-frontend` | Faucet SPA | none |
| `explorer` | Explorer SPA | none |
| **`edge`** | nginx reverse proxy + TLS | **`:80`, `:443`** |
| **`certbot`** | LE issue/renew (`CERTBOT_AUTH`) or idle if disabled | none |

Configs: [nginx/dew-edge.docker.conf](./nginx/dew-edge.docker.conf), [nginx/certbot-entrypoint.sh](./nginx/certbot-entrypoint.sh).

| TLS mode | When | Key settings |
| :--- | :--- | :--- |
| **DNS-01 Cloudflare** ★ | Orange-cloud, real LE on origin | `CERTBOT_AUTH=dns-cloudflare`, `CERTBOT_IMAGE=certbot/dns-cloudflare`, `CLOUDFLARE_API_TOKEN` |
| **HTTP-01 webroot** | No CF / grey-cloud | `CERTBOT_AUTH=webroot`, inbound `:80` |
| **Origin CA** (optional) | No LE on VPS | `CERTBOT_DISABLE=1` + mount PEMs at `/etc/nginx/origin-certs` |

**Edge cert priority:** optional origin PEMs (if you mount them) → Let's Encrypt → bootstrap self-signed. Edge reloads within ~60s when LE appears.

**Optional Origin CA volume** (only if not using certbot LE):

```yaml
# under edge.volumes in a compose override, or edit docker-compose.yml:
- /path/on/host/ssl:/etc/nginx/origin-certs:ro
```

### Alternative: host nginx (no Compose edge)

If you terminate TLS on the host instead of the `edge` container:

```bash
# backends must listen on 127.0.0.1:8545 / 8081 / 8082 (not the default compose layout)
sudo bash deploy/scripts/install-edge-nginx.sh
export CERTBOT_EMAIL=ops@fadosoft.com
sudo -E bash deploy/scripts/setup-certbot.sh
```

Do **not** run host nginx and Compose `edge` on the same host ports at once.

### Wildcard / multi-name cert note

Hostnames follow the `*-dew.fadosoft.com` naming convention (`rpc-dew`, `faucet-dew`, `explorer-dew`). That is **not** a DNS wildcard label — Let's Encrypt covers them via a **multi-name (SAN)** cert (default). A true zone wildcard (`*.fadosoft.com`) needs **DNS-01**.

## systemd (binary install)

See [systemd/dew.service](./node/systemd/dew.service) (general) or [dew-rpc-public.service](./node/systemd/dew-rpc-public.service) (path B). Create user/dirs:

```bash
sudo useradd --system --home /var/lib/dew --shell /usr/sbin/nologin dew || true
sudo mkdir -p /var/lib/dew /etc/dew
sudo install -m 755 bin/dew /usr/local/bin/dew
sudo install -m 644 genesis.json /etc/dew/genesis.json
sudo chown -R dew:dew /var/lib/dew
sudo install -m 644 deploy/node/systemd/dew.service /etc/systemd/system/dew.service
sudo systemctl daemon-reload
sudo systemctl enable --now dew
```

## Faucet Deployment (Dockerized)

See [faucet/docker-compose.yml](./faucet/docker-compose.yml) to deploy Faucet backend and frontend under a single host.

```bash
cp deploy/faucet/faucet.env.example deploy/faucet/faucet.env
# edit deploy/faucet/faucet.env with your keys, chain details, and recaptcha config
docker compose -f deploy/faucet/docker-compose.yml up --build -d
```

Frontend runs on `:8081` by default and routes backend requests internally.

## Explorer Deployment (Dockerized)

Static SPA only — no backend, no keys. Browser calls `PUBLIC_RPC_URL` directly (must be CORS-reachable). Spec: [block-explorer.md](../docs/product/block-explorer.md).

```bash
cp deploy/explorer/explorer.env.example deploy/explorer/explorer.env
# edit PUBLIC_RPC_URL (public HTTPS RPC) and PUBLIC_EXPLORER_BASE
docker compose -f deploy/explorer/docker-compose.yml --env-file deploy/explorer/explorer.env up --build -d
```

Default host port: **`:8082`**. Rebuild the image when env changes (Vite bakes `PUBLIC_*` at build time).

Smoke:

1. Open `http://localhost:8082/` — home stats + latest blocks  
2. Deep links: `/block/{n}`, `/tx/{hash}`, `/address/{addr}`  
3. Set faucet `PUBLIC_EXPLORER_URL` to the same base for “View on block explorer” links  

## Combined Node + Faucet + Explorer (Docker Compose)

Full stack with **edge nginx** on `:80`/`:443`. Backends are not published on the host.

```bash
cp deploy/.env.example deploy/.env   # set CLOUDFLARE_API_TOKEN + CERTBOT_* for DNS-01
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up --build -d
docker compose -f deploy/docker-compose.yml logs -f certbot edge
```

| Surface | Public URL |
| :--- | :--- |
| JSON-RPC | `https://rpc-dew.fadosoft.com` |
| Faucet web | `https://faucet-dew.fadosoft.com` |
| Faucet API | `https://faucet-dew.fadosoft.com/api` |
| Explorer | `https://explorer-dew.fadosoft.com` |

Local smoke **before** DNS/TLS (Host header + edge :80; HTTPS redirects until real certs):

```bash
curl -sk -X POST https://127.0.0.1/ -H 'Host: rpc-dew.fadosoft.com' \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

`PUBLIC_RPC_URL` must be **browser-reachable** (`https://rpc-dew.fadosoft.com`), not Docker-internal `http://dew-node:8545`.

## Related residual

- Persistent peer store / auto-redial after restart — `agents/debt.md` (C5)
- Production faucet — D2: `cmd/dewfaucet`, [docs/product/faucet.md](../docs/product/faucet.md), [faucet.env.example](./faucet/faucet.env.example), [systemd/dewfaucet.service](./faucet/systemd/dewfaucet.service)
- Block explorer packaging — D1: [explorer/](./explorer/), [block-explorer.md](../docs/product/block-explorer.md)
- Public bootnode hostnames — C6 ops residual

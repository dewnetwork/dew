# Controlled public RPC — single host (path B)

Fastest public surface after private soak: **one host**, **one JSON-RPC**, **TLS + rate limit**, no multi-validator public mesh yet.

Freeze: **`public-testnet-v1`** · chain ID **`2205`**.

| Do | Do not |
| :--- | :--- |
| Keep Dew off the public interface | Expose `:8545` on `0.0.0.0` to the internet |
| Terminate HTTP(S) on 80/443 (nginx) | Ship Anvil / staging keys as faucet or “validator” |
| Rate-limit POST body / RPS | Open staking (`--staking`) without a plan |
| Keep emergency stop: stop proxy first | Promise mainnet or economic incentives |

This is a **public demo RPC**, not a multi-host BFT network. Clients share one auto-mine / in-process stack.

---

## 0. Inventory

| Item | Example |
| :--- | :------ |
| Host | Ubuntu 22.04/24.04, 1–2 vCPU, 2 GB RAM |
| Domain (optional but recommended) | `rpc.example.com` → A-record to this host |
| Public URL | `https://rpc.example.com` (or `http://…` until TLS) |
| Dew RPC | internal only (Compose) or `http://127.0.0.1:8545` (systemd) |

Firewall (ufw):

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
# do NOT: ufw allow 8545
sudo ufw enable
```

---

## 1. Docker Compose (recommended packaging)

From **repo root** after private soak:

```bash
# Stop soak if still running
docker compose -f deploy/docker-compose.soak.yml --env-file deploy/soak.env down

cp deploy/public.env.example deploy/public.env   # once
docker compose -f deploy/docker-compose.yml --env-file deploy/public.env up --build
```

| Container | Role | Host ports |
| :--- | :--- | :--- |
| `dew-rpc` | `dew run` on network `dew_internal` only | none |
| `dew-rpc-proxy` | nginx → `dew:8545` (rate limit, 1m body, POST/OPTIONS) | `:80`, `:443` |

Smoke (HTTP before TLS):

```bash
node scripts/smoke-rpc.mjs http://127.0.0.1
# expect chainId 2205 / 0x89d
```

Config files:

- [docker-compose.yml](./docker-compose.yml)
- [nginx/dew-rpc.docker.conf](./nginx/dew-rpc.docker.conf) — upstream `dew:8545`

### TLS with Compose

1. Place `fullchain.pem` + `privkey.pem` under `deploy/certs/`.
2. Uncomment the `./certs` volume on `proxy` in `docker-compose.yml`.
3. Uncomment the HTTPS `server` block in `nginx/dew-rpc.docker.conf`.
4. Recreate proxy:  
   `docker compose -f deploy/docker-compose.yml up -d --force-recreate proxy`

### Ops (Compose)

| Action | Command |
| :--- | :--- |
| Logs (node) | `docker logs -f dew-rpc` |
| Logs (proxy) | `docker logs -f dew-rpc-proxy` |
| Restart node | `docker compose -f deploy/docker-compose.yml restart dew` |
| Stop public surface | `docker compose -f deploy/docker-compose.yml stop proxy` |
| Full stop | `docker compose -f deploy/docker-compose.yml down` |

---

## 2. Binary + systemd (no Docker)

On a build machine or the host (needs Go 1.23+):

```bash
git clone <your-fork-or-repo> dewchain && cd dewchain
go build -o bin/dew ./cmd/dew
sudo useradd --system --home /var/lib/dew --shell /usr/sbin/nologin dew || true
sudo mkdir -p /var/lib/dew /etc/dew
sudo install -m 755 bin/dew /usr/local/bin/dew
sudo install -m 644 genesis.json /etc/dew/genesis.json
sudo chown -R dew:dew /var/lib/dew
```

**Genesis:** use the freeze sample or a dedicated public genesis. Same chain ID **2205**. If you re-alloc faucet, use a **new** funded key — not Anvil #0 on a long-lived public endpoint.

### systemd — listen only on loopback

```bash
sudo install -m 644 deploy/systemd/dew-rpc-public.service /etc/systemd/system/dew.service
sudo systemctl daemon-reload
sudo systemctl enable --now dew
sudo systemctl status dew --no-pager
```

Smoke **on the host**:

```bash
curl -s -X POST http://127.0.0.1:8545 \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
# expect "result":"0x89d"
```

Unit runs `dew run` (single execution node, auto-mine per tx). For in-process 3-validator demo instead, change `ExecStart` to `dew devnet --http.addr 127.0.0.1 --http.port 8545` (heavier; still one host).

### TLS reverse proxy + rate limit (host nginx + certbot)

```bash
sudo apt-get update && sudo apt-get install -y nginx certbot python3-certbot-nginx
sudo install -m 644 deploy/nginx/dew-rpc.conf /etc/nginx/sites-available/dew-rpc
sudo ln -sf /etc/nginx/sites-available/dew-rpc /etc/nginx/sites-enabled/dew-rpc
# edit server_name (rpc.example.com → your domain)
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d rpc.example.com
```

nginx sample (`deploy/nginx/dew-rpc.conf`) includes:

- Proxy → `http://127.0.0.1:8545`
- Only `POST` / `OPTIONS` (MetaMask preflight)
- Rate limit ~**10 r/s** burst 20 per IP (tune if abused)
- Body size **1m** (matches Dew `MaxRequestBodyBytes`)

### Ops (systemd)

| Action | Command |
| :--- | :--- |
| Logs | `journalctl -u dew -f` |
| Restart node | `sudo systemctl restart dew` |
| Stop public surface | `sudo systemctl stop nginx` (node can stay on loopback) |
| Full stop | `sudo systemctl stop dew` |

---

## 3. Publish (minimal)

```text
Network:     Dew public-testnet-v1
Chain ID:    2205
RPC:         https://rpc.example.com
Symbol:      DEW
Explorer:    (none)
Faucet:      (none / manual / allowlist only)
Bootnodes:   (n/a — single RPC path B)
```

MetaMask: Custom network → RPC URL HTTPS, chain ID **2205**, symbol **DEW**.

External smoke:

```bash
node scripts/smoke-rpc.mjs https://rpc.example.com
```

---

## 4. While live

Monitor: 5xx from proxy, container/`journalctl` panics, disk, CPU. Dew already rejects oversized body/batch (C6).

**Faucet:** optional and separate. Prefer allowlist DM / form; never paste Anvil keys on a public page.

**Staking:** leave default **off** (Compose and `dew-rpc-public.service` have no `--staking`).

---

## 5. Emergency stop

1. Stop the **proxy** first — Compose: `docker compose -f deploy/docker-compose.yml stop proxy` · systemd: `sudo systemctl stop nginx`  
2. If node bug: stop Dew (`docker compose … stop dew` / `systemctl stop dew`)  
3. Investigate; do not publish `:8545` on the public interface  

---

## Related

- [Launch checklist](../docs/development/launch-checklist.md)  
- [Public testnet freeze](../docs/development/public-testnet.md)  
- [deploy/README](./README.md)  
- [docker-compose.yml](./docker-compose.yml)  
- [systemd/dew-rpc-public.service](./systemd/dew-rpc-public.service)  
- [nginx/dew-rpc.conf](./nginx/dew-rpc.conf)  
- [nginx/dew-rpc.docker.conf](./nginx/dew-rpc.docker.conf)  

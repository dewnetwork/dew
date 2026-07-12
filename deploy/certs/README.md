# Edge TLS secrets (host-mounted)

This directory is mounted into the Compose stack. **Never commit tokens or private keys** (see `.gitignore`).

| File | Used by |
| :--- | :--- |
| `fullchain.pem` + `privkey.pem` | Edge origin TLS (Cloudflare Origin CA, etc.) |
| `cloudflare.ini` | Certbot **DNS-01** (`CERTBOT_AUTH=dns-cloudflare`) |

---

## Option A — Certbot DNS-01 (Cloudflare) ★ recommended with orange-cloud

Let's Encrypt proves domain ownership via a **TXT record** that certbot creates with the Cloudflare API.  
Works with **Proxied** DNS; does **not** need public HTTP-01 or open `:80` to the world for ACME.

### 1. Cloudflare API token

1. [Create API token](https://dash.cloudflare.com/profile/api-tokens) → **Create Token**
2. Template **Edit zone DNS**, or custom:
   - Permissions: **Zone → DNS → Edit**
   - Zone Resources: **Include → Specific zone →** `fadosoft.com` (or your zone)
3. Copy the token once.

### 2. Credentials file

```bash
# deploy/certs/cloudflare.ini  (chmod 600)
dns_cloudflare_api_token = YOUR_TOKEN_HERE
```

Do **not** use Global API Key in new setups.

### 3. `.env`

```bash
CERTBOT_AUTH=dns-cloudflare
CERTBOT_EMAIL=ops@fadosoft.com
# Image that includes the Cloudflare DNS plugin:
CERTBOT_IMAGE=certbot/dns-cloudflare:v2.11.0
# Optional: wait longer for TXT propagation (default 30)
# CERTBOT_DNS_PROPAGATION_SECONDS=60
# Do NOT set CERTBOT_DISABLE
# Leave fullchain.pem / privkey.pem absent (or remove them) so edge uses LE
```

### 4. Cloudflare SSL mode

| Setting | Value |
| :--- | :--- |
| DNS | **Proxied** (orange) OK |
| SSL/TLS | **Full (strict)** after LE is on origin |

### 5. Apply

```bash
chmod 600 deploy/certs/cloudflare.ini
docker compose -f deploy/docker-compose.yml --env-file deploy/.env up -d edge certbot
docker logs -f dew-certbot
# expect: issue succeeded
docker logs dew-edge 2>&1 | head -20
# expect: edge: using Let's Encrypt from …
```

Renewal is automatic (same DNS-01 plugin, every 12h).

---

## Option B — Cloudflare Origin Certificate (no LE on VPS)

1. CF → **SSL/TLS** → **Origin Server** → **Create certificate**
2. Save as `fullchain.pem` + `privkey.pem` here
3. `.env`: `CERTBOT_DISABLE=1`
4. CF SSL mode: **Full (strict)**; DNS Proxied

See also main [deploy/README.md](../README.md).

---

## Option C — Certbot HTTP-01 (webroot)

```bash
CERTBOT_AUTH=webroot   # default
CERTBOT_EMAIL=ops@…
CERTBOT_IMAGE=certbot/certbot:v2.11.0
```

Needs inbound **:80** and DNS that reaches origin (grey-cloud if CF 522 on HTTP-01).

---

## Comparison

| | DNS-01 Cloudflare | Origin Cert | HTTP-01 webroot |
| :--- | :--- | :--- | :--- |
| Orange-cloud | ✅ | ✅ | ❌ often 522 |
| Real LE browser trust on origin | ✅ | ❌ (CF-only trust) | ✅ |
| Needs CF API token | ✅ | ❌ | ❌ |
| Auto-renew on VPS | ✅ | N/A (long-lived) | ✅ |
| Wildcard `*.dew…` | ✅ (add `-d`) | ✅ | ❌ (needs DNS-01) |

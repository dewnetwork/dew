#!/usr/bin/env bash
# Manual force issue / renew for the Compose edge stack, then reload nginx.
#
# Normal path: the `certbot` service in deploy/docker-compose.yml auto-issues on
# first boot (when no lineage exists) and renews every 12h. Use this script to
# force a certonly run without waiting for the container retry loop.
#
# Prerequisites:
#   - DNS A/AAAA for the three hostnames → this host
#   - Ports 80/443 free on the host (edge publishes them)
#   - Stack running: docker compose -f deploy/docker-compose.yml up -d
#
# Usage (from repo root):
#   export CERTBOT_EMAIL=ops@fadosoft.com
#   bash deploy/scripts/setup-certbot-docker.sh
#
# Host nginx path (alternative): deploy/scripts/setup-certbot.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
COMPOSE_FILE="${COMPOSE_FILE:-${ROOT}/deploy/docker-compose.yml}"
ENV_FILE="${ENV_FILE:-${ROOT}/deploy/.env}"

RPC_HOST="${RPC_HOST:-rpc-dew.fadosoft.com}"
FAUCET_HOST="${FAUCET_HOST:-faucet-dew.fadosoft.com}"
EXPLORER_HOST="${EXPLORER_HOST:-explorer-dew.fadosoft.com}"
CERT_PRIMARY="${CERT_PRIMARY:-${RPC_HOST}}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"

COMPOSE=(docker compose -f "${COMPOSE_FILE}")
if [[ -f "${ENV_FILE}" ]]; then
  COMPOSE+=(--env-file "${ENV_FILE}")
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker not found" >&2
  exit 1
fi

# Ensure edge + certbot volumes are up (edge must answer ACME on :80)
echo "Ensuring edge is running…"
"${COMPOSE[@]}" up -d edge

# Wait briefly for nginx
for _ in $(seq 1 30); do
  if "${COMPOSE[@]}" exec -T edge wget -q -O /dev/null http://127.0.0.1/ 2>/dev/null \
    || "${COMPOSE[@]}" exec -T edge true 2>/dev/null; then
    break
  fi
  sleep 1
done

CERTBOT_ARGS=(
  certonly
  --webroot
  -w /var/www/certbot
  -d "${RPC_HOST}"
  -d "${FAUCET_HOST}"
  -d "${EXPLORER_HOST}"
  --cert-name "${CERT_PRIMARY}"
  --agree-tos
  --non-interactive
  --keep-until-expiring
)

if [[ -n "${CERTBOT_EMAIL}" ]]; then
  CERTBOT_ARGS+=(-m "${CERTBOT_EMAIL}")
else
  CERTBOT_ARGS+=(--register-unsafely-without-email)
  echo "warn: CERTBOT_EMAIL unset — using --register-unsafely-without-email" >&2
fi

echo "Requesting certificates for ${RPC_HOST}, ${FAUCET_HOST}, ${EXPLORER_HOST}…"
"${COMPOSE[@]}" run --rm certbot "${CERTBOT_ARGS[@]}"

# Force edge to re-copy LE material and reload
echo "Reloading edge nginx…"
"${COMPOSE[@]}" exec -T edge /bin/sh -c '
  CERT_PRIMARY="${CERT_PRIMARY:-rpc-dew.fadosoft.com}"
  LE_DIR="/etc/letsencrypt/live/'"${CERT_PRIMARY}"'"
  SSL_DIR=/etc/nginx/ssl
  if [ -f "$LE_DIR/fullchain.pem" ] && [ -f "$LE_DIR/privkey.pem" ]; then
    cp -L "$LE_DIR/fullchain.pem" "$SSL_DIR/fullchain.pem"
    cp -L "$LE_DIR/privkey.pem" "$SSL_DIR/privkey.pem"
    nginx -s reload
    echo "reloaded with Let'\''s Encrypt certs"
  else
    echo "error: cert files missing under $LE_DIR" >&2
    exit 1
  fi
'

echo
echo "TLS ready (Compose edge):"
echo "  https://${RPC_HOST}"
echo "  https://${FAUCET_HOST}"
echo "  https://${EXPLORER_HOST}"
echo
echo "Auto-issue + renew: certbot service in compose (issue if missing; renew every 12h)."
echo "Logs:          ${COMPOSE[*]} logs -f certbot edge"
echo "Smoke RPC:     node scripts/smoke-rpc.mjs https://${RPC_HOST}"

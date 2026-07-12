#!/usr/bin/env bash
# Issue Let's Encrypt certs for Dew public edge (*.dew.fadosoft.com names) and
# enable certbot auto-renew. Requires host nginx already serving :80 for these
# hostnames (install deploy/nginx/dew-edge.conf first).
#
# Usage (from repo root, on the public host):
#   export CERTBOT_EMAIL=ops@fadosoft.com   # required first run
#   sudo bash deploy/scripts/setup-certbot.sh
#
# DNS (before running):
#   A/AAAA  rpc.dew.fadosoft.com       → this host
#   A/AAAA  faucet.dew.fadosoft.com    → this host
#   A/AAAA  explorer.dew.fadosoft.com  → this host
#
# Wildcard (*.dew.fadosoft.com) needs DNS-01, not this script — see deploy/README.md.

set -euo pipefail

RPC_HOST="${RPC_HOST:-rpc.dew.fadosoft.com}"
FAUCET_HOST="${FAUCET_HOST:-faucet.dew.fadosoft.com}"
EXPLORER_HOST="${EXPLORER_HOST:-explorer.dew.fadosoft.com}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "error: run as root (sudo bash deploy/scripts/setup-certbot.sh)" >&2
  exit 1
fi

if ! command -v nginx >/dev/null 2>&1; then
  echo "error: nginx not installed. Install edge first:" >&2
  echo "  apt-get install -y nginx certbot python3-certbot-nginx" >&2
  exit 1
fi

if ! command -v certbot >/dev/null 2>&1; then
  apt-get update
  apt-get install -y certbot python3-certbot-nginx
fi

nginx -t
systemctl enable --now nginx

CERTBOT_ARGS=(
  --nginx
  -d "${RPC_HOST}"
  -d "${FAUCET_HOST}"
  -d "${EXPLORER_HOST}"
  --agree-tos
  --redirect
  --non-interactive
)

if [[ -n "${CERTBOT_EMAIL}" ]]; then
  CERTBOT_ARGS+=(-m "${CERTBOT_EMAIL}")
else
  CERTBOT_ARGS+=(--register-unsafely-without-email)
  echo "warn: CERTBOT_EMAIL unset — using --register-unsafely-without-email" >&2
fi

echo "Requesting certificates for ${RPC_HOST}, ${FAUCET_HOST}, ${EXPLORER_HOST}…"
certbot "${CERTBOT_ARGS[@]}"

# Auto-renew (Ubuntu/Debian package provides certbot.timer)
if systemctl list-unit-files certbot.timer >/dev/null 2>&1; then
  systemctl enable --now certbot.timer
  systemctl list-timers certbot.timer --no-pager || true
else
  # Fallback renew cron if timer unit missing
  CRON_LINE='0 3 * * * root certbot renew --quiet --deploy-hook "systemctl reload nginx"'
  if [[ ! -f /etc/cron.d/dew-certbot-renew ]]; then
    echo "${CRON_LINE}" >/etc/cron.d/dew-certbot-renew
    chmod 644 /etc/cron.d/dew-certbot-renew
    echo "installed /etc/cron.d/dew-certbot-renew"
  fi
fi

# Ensure nginx reloads after renew (idempotent deploy hook)
HOOK_DIR=/etc/letsencrypt/renewal-hooks/deploy
mkdir -p "${HOOK_DIR}"
cat >"${HOOK_DIR}/reload-nginx.sh" <<'HOOK'
#!/bin/sh
systemctl reload nginx
HOOK
chmod 755 "${HOOK_DIR}/reload-nginx.sh"

echo
echo "TLS ready:"
echo "  https://${RPC_HOST}"
echo "  https://${FAUCET_HOST}"
echo "  https://${EXPLORER_HOST}"
echo
echo "Dry-run renew: certbot renew --dry-run"
echo "Smoke RPC:     node scripts/smoke-rpc.mjs https://${RPC_HOST}"

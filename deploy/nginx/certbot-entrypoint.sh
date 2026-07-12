#!/bin/sh
# Certbot compose entrypoint: issue Let's Encrypt once if missing, then renew forever.
#
# HTTP-01 via shared webroot with the edge nginx container.
# Edge materializes /etc/letsencrypt/live/$CERT_PRIMARY into nginx SSL paths and reloads.
#
# Env (see deploy/.env.example):
#   CERTBOT_EMAIL          — recommended (Let's Encrypt account)
#   RPC_HOST / FAUCET_HOST / EXPLORER_HOST — SANs (defaults: *.dew.fadosoft.com)
#   CERT_PRIMARY           — certbot --cert-name / live/ directory name
#   CERTBOT_STAGING=1      — use LE staging (rate-limit safe)
#   CERTBOT_ISSUE_RETRY_SEC — seconds between failed issue attempts (default 120)
#   CERTBOT_RENEW_INTERVAL_SEC — renew loop sleep (default 43200 = 12h)
set -eu

WEBROOT="${CERTBOT_WEBROOT:-/var/www/certbot}"
RPC_HOST="${RPC_HOST:-rpc.dew.fadosoft.com}"
FAUCET_HOST="${FAUCET_HOST:-faucet.dew.fadosoft.com}"
EXPLORER_HOST="${EXPLORER_HOST:-explorer.dew.fadosoft.com}"
CERT_PRIMARY="${CERT_PRIMARY:-${RPC_HOST}}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"
CERTBOT_STAGING="${CERTBOT_STAGING:-}"
ISSUE_RETRY_SEC="${CERTBOT_ISSUE_RETRY_SEC:-120}"
RENEW_INTERVAL_SEC="${CERTBOT_RENEW_INTERVAL_SEC:-43200}"

LE_LIVE="/etc/letsencrypt/live/${CERT_PRIMARY}"

has_cert() {
  [ -f "${LE_LIVE}/fullchain.pem" ] && [ -f "${LE_LIVE}/privkey.pem" ]
}

issue_cert() {
  echo "certbot: requesting certificate for ${RPC_HOST}, ${FAUCET_HOST}, ${EXPLORER_HOST} (cert-name=${CERT_PRIMARY})"
  set -- certonly \
    --webroot -w "${WEBROOT}" \
    -d "${RPC_HOST}" \
    -d "${FAUCET_HOST}" \
    -d "${EXPLORER_HOST}" \
    --cert-name "${CERT_PRIMARY}" \
    --agree-tos \
    --non-interactive \
    --keep-until-expiring
  if [ -n "${CERTBOT_EMAIL}" ]; then
    set -- "$@" -m "${CERTBOT_EMAIL}"
  else
    echo "certbot: warn: CERTBOT_EMAIL unset — using --register-unsafely-without-email" >&2
    set -- "$@" --register-unsafely-without-email
  fi
  if [ "${CERTBOT_STAGING}" = "1" ] || [ "${CERTBOT_STAGING}" = "true" ]; then
    echo "certbot: using Let's Encrypt staging" >&2
    set -- "$@" --staging
  fi
  certbot "$@"
}

echo "certbot: ensuring webroot ${WEBROOT}"
mkdir -p "${WEBROOT}"

# depends_on edge is not a readiness probe — brief wait for nginx :80
echo "certbot: waiting for edge to accept ACME (webroot HTTP-01)…"
i=0
while [ "${i}" -lt 60 ]; do
  # Webroot is a shared volume; edge must be listening on :80 for the public challenge.
  # We cannot curl the public host from here reliably; short sleep is enough after depends_on.
  i=$((i + 1))
  if [ "${i}" -ge 3 ]; then
    break
  fi
  sleep 2
done

if has_cert; then
  echo "certbot: existing cert at ${LE_LIVE}"
else
  echo "certbot: no cert yet — issuing (retry every ${ISSUE_RETRY_SEC}s until success)"
  echo "certbot: requires DNS A/AAAA → this host and inbound :80 (Cloudflare: DNS only or allow ACME)"
  while ! has_cert; do
    if issue_cert; then
      echo "certbot: issue succeeded — edge reloads within ~60s"
      break
    fi
    echo "certbot: issue failed — retry in ${ISSUE_RETRY_SEC}s" >&2
    sleep "${ISSUE_RETRY_SEC}" &
    wait $! || true
  done
fi

echo "certbot: renew loop every ${RENEW_INTERVAL_SEC}s"
while true; do
  if has_cert; then
    certbot renew --webroot -w "${WEBROOT}" --quiet || echo "certbot: renew failed" >&2
  else
    issue_cert || echo "certbot: issue retry failed" >&2
  fi
  sleep "${RENEW_INTERVAL_SEC}" &
  wait $! || true
done

#!/bin/sh
# Certbot compose entrypoint: issue Let's Encrypt once if missing, then renew forever.
#
# Authenticators (CERTBOT_AUTH):
#   webroot         — HTTP-01 via shared volume with edge (default)
#   dns-cloudflare  — DNS-01 via Cloudflare API (works with orange-cloud proxy)
#
# DNS-01 credentials: CLOUDFLARE_API_TOKEN in deploy/.env (required for dns-cloudflare).
# Disable: CERTBOT_DISABLE=1
#
# Env: see deploy/.env.example and deploy/README.md
set -eu

if [ "${CERTBOT_DISABLE:-}" = "1" ] || [ "${CERTBOT_DISABLE:-}" = "true" ]; then
  echo "certbot: CERTBOT_DISABLE set — skipping issue/renew (origin certs or external TLS)"
  while true; do
    sleep 86400 &
    wait $! || true
  done
fi

WEBROOT="${CERTBOT_WEBROOT:-/var/www/certbot}"
RPC_HOST="${RPC_HOST:-rpc-dew.fadosoft.com}"
FAUCET_HOST="${FAUCET_HOST:-faucet-dew.fadosoft.com}"
EXPLORER_HOST="${EXPLORER_HOST:-explorer-dew.fadosoft.com}"
GUESTBOOK_HOST="${GUESTBOOK_HOST:-guestbook-dew.fadosoft.com}"
CERT_PRIMARY="${CERT_PRIMARY:-${RPC_HOST}}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-}"
CERTBOT_STAGING="${CERTBOT_STAGING:-}"
CERTBOT_AUTH="${CERTBOT_AUTH:-webroot}"
CF_TOKEN="${CLOUDFLARE_API_TOKEN:-}"
DNS_PROPAGATION="${CERTBOT_DNS_PROPAGATION_SECONDS:-30}"
ISSUE_RETRY_SEC="${CERTBOT_ISSUE_RETRY_SEC:-120}"
RENEW_INTERVAL_SEC="${CERTBOT_RENEW_INTERVAL_SEC:-43200}"

LE_LIVE="/etc/letsencrypt/live/${CERT_PRIMARY}"

has_cert() {
  [ -f "${LE_LIVE}/fullchain.pem" ] && [ -f "${LE_LIVE}/privkey.pem" ]
}

# Plugin needs a mode-600 ini file; build one from CLOUDFLARE_API_TOKEN.
prepare_cloudflare_creds() {
  if [ -z "${CF_TOKEN}" ]; then
    echo "certbot: error: CLOUDFLARE_API_TOKEN is unset (required for CERTBOT_AUTH=dns-cloudflare)" >&2
    echo "certbot: set it in deploy/.env — Cloudflare token with Zone → DNS → Edit" >&2
    return 1
  fi
  CREDS_USE="/tmp/cloudflare.ini"
  umask 077
  printf 'dns_cloudflare_api_token = %s\n' "${CF_TOKEN}" >"${CREDS_USE}"
  chmod 600 "${CREDS_USE}"
  echo "certbot: using Cloudflare API token from CLOUDFLARE_API_TOKEN env"
  return 0
}

run_certbot() {
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

issue_cert() {
  echo "certbot: requesting certificate for ${RPC_HOST}, ${FAUCET_HOST}, ${EXPLORER_HOST}, ${GUESTBOOK_HOST} (cert-name=${CERT_PRIMARY}, auth=${CERTBOT_AUTH})"

  case "${CERTBOT_AUTH}" in
    webroot)
      run_certbot certonly \
        --webroot -w "${WEBROOT}" \
        -d "${RPC_HOST}" \
        -d "${FAUCET_HOST}" \
        -d "${EXPLORER_HOST}" \
        -d "${GUESTBOOK_HOST}" \
        --cert-name "${CERT_PRIMARY}" \
        --agree-tos \
        --non-interactive \
        --keep-until-expiring
      ;;
    dns-cloudflare | cloudflare | dns)
      prepare_cloudflare_creds || return 1
      run_certbot certonly \
        --dns-cloudflare \
        --dns-cloudflare-credentials "${CREDS_USE}" \
        --dns-cloudflare-propagation-seconds "${DNS_PROPAGATION}" \
        -d "${RPC_HOST}" \
        -d "${FAUCET_HOST}" \
        -d "${EXPLORER_HOST}" \
        -d "${GUESTBOOK_HOST}" \
        --cert-name "${CERT_PRIMARY}" \
        --agree-tos \
        --non-interactive \
        --keep-until-expiring
      ;;
    *)
      echo "certbot: error: unknown CERTBOT_AUTH=${CERTBOT_AUTH} (use webroot or dns-cloudflare)" >&2
      return 1
      ;;
  esac
}

renew_cert() {
  case "${CERTBOT_AUTH}" in
    webroot)
      certbot renew --webroot -w "${WEBROOT}" --quiet
      ;;
    *)
      certbot renew --quiet
      ;;
  esac
}

echo "certbot: auth=${CERTBOT_AUTH}"

if [ "${CERTBOT_AUTH}" = "webroot" ]; then
  echo "certbot: ensuring webroot ${WEBROOT}"
  mkdir -p "${WEBROOT}"
  echo "certbot: waiting briefly for edge (HTTP-01)…"
  sleep 6
else
  echo "certbot: DNS-01 — no HTTP webroot required (Cloudflare orange-cloud OK)"
  sleep 2
fi

if has_cert; then
  echo "certbot: existing cert at ${LE_LIVE}"
else
  echo "certbot: no cert yet — issuing (retry every ${ISSUE_RETRY_SEC}s until success)"
  case "${CERTBOT_AUTH}" in
    webroot)
      echo "certbot: requires DNS → this host and inbound :80 (or grey-cloud if behind Cloudflare)"
      ;;
    dns-cloudflare | cloudflare | dns)
      echo "certbot: requires CLOUDFLARE_API_TOKEN in .env (Zone.DNS Edit)"
      ;;
  esac
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
    renew_cert || echo "certbot: renew failed" >&2
  else
    issue_cert || echo "certbot: issue retry failed" >&2
  fi
  sleep "${RENEW_INTERVAL_SEC}" &
  wait $! || true
done

#!/bin/sh
# Edge nginx entrypoint: materialize TLS certs then start nginx.
# Prefer Let's Encrypt lineage; otherwise bootstrap a short-lived self-signed cert
# so :443 can bind before the first certbot run.
set -eu

CERT_PRIMARY="${CERT_PRIMARY:-rpc.dew.fadosoft.com}"
LE_DIR="/etc/letsencrypt/live/${CERT_PRIMARY}"
SSL_DIR=/etc/nginx/ssl
HASH_FILE="${SSL_DIR}/.materialized.hash"

mkdir -p "${SSL_DIR}" /var/www/certbot

materialize_certs() {
  if [ -f "${LE_DIR}/fullchain.pem" ] && [ -f "${LE_DIR}/privkey.pem" ]; then
    # -L: follow certbot archive symlinks
    cp -L "${LE_DIR}/fullchain.pem" "${SSL_DIR}/fullchain.pem"
    cp -L "${LE_DIR}/privkey.pem" "${SSL_DIR}/privkey.pem"
    return 0
  fi

  if [ -f "${SSL_DIR}/fullchain.pem" ] && [ -f "${SSL_DIR}/privkey.pem" ]; then
    return 0
  fi

  echo "edge: no Let's Encrypt cert yet — generating bootstrap self-signed for ${CERT_PRIMARY}"
  if ! command -v openssl >/dev/null 2>&1; then
    apk add --no-cache openssl >/dev/null
  fi
  openssl req -x509 -nodes -newkey rsa:2048 -days 3 \
    -keyout "${SSL_DIR}/privkey.pem" \
    -out "${SSL_DIR}/fullchain.pem" \
    -subj "/CN=${CERT_PRIMARY}" >/dev/null 2>&1
}

certs_hash() {
  if [ -f "${SSL_DIR}/fullchain.pem" ] && [ -f "${SSL_DIR}/privkey.pem" ]; then
    cat "${SSL_DIR}/fullchain.pem" "${SSL_DIR}/privkey.pem" | sha256sum | awk '{print $1}'
  else
    echo none
  fi
}

materialize_certs
certs_hash >"${HASH_FILE}"

# Pick up new/renewed LE certs without docker socket (certbot runs in sibling container).
# Poll often so the first auto-issue lands within ~1 minute after certbot succeeds.
(
  while true; do
    sleep 60
    materialize_certs || true
    new_hash="$(certs_hash)"
    old_hash="$(cat "${HASH_FILE}" 2>/dev/null || echo none)"
    if [ "${new_hash}" != "${old_hash}" ]; then
      echo "edge: TLS material changed — reloading nginx"
      echo "${new_hash}" >"${HASH_FILE}"
      nginx -s reload || true
    fi
  done
) &

exec nginx -g "daemon off;"

#!/bin/sh
# Edge nginx entrypoint: materialize TLS certs then start nginx.
#
# Priority:
#   1) Host-mounted origin certs (/etc/nginx/origin-certs) — e.g. Cloudflare Origin CA
#   2) Let's Encrypt lineage (/etc/letsencrypt/live/$CERT_PRIMARY) — webroot or DNS-01
#   3) Short-lived bootstrap self-signed so :443 can bind before first cert
set -eu

CERT_PRIMARY="${CERT_PRIMARY:-rpc-dew.fadosoft.com}"
ORIGIN_DIR="${ORIGIN_CERT_DIR:-/etc/nginx/origin-certs}"
LE_DIR="/etc/letsencrypt/live/${CERT_PRIMARY}"
SSL_DIR=/etc/nginx/ssl
HASH_FILE="${SSL_DIR}/.materialized.hash"

mkdir -p "${SSL_DIR}" /var/www/certbot

copy_pair() {
  src_dir="$1"
  label="$2"
  cp -L "${src_dir}/fullchain.pem" "${SSL_DIR}/fullchain.pem"
  cp -L "${src_dir}/privkey.pem" "${SSL_DIR}/privkey.pem"
  echo "edge: using ${label} from ${src_dir}"
}

materialize_certs() {
  if [ -f "${ORIGIN_DIR}/fullchain.pem" ] && [ -f "${ORIGIN_DIR}/privkey.pem" ]; then
    copy_pair "${ORIGIN_DIR}" "origin certs"
    return 0
  fi

  if [ -f "${LE_DIR}/fullchain.pem" ] && [ -f "${LE_DIR}/privkey.pem" ]; then
    copy_pair "${LE_DIR}" "Let's Encrypt"
    return 0
  fi

  if [ -f "${SSL_DIR}/fullchain.pem" ] && [ -f "${SSL_DIR}/privkey.pem" ]; then
    return 0
  fi

  echo "edge: no origin/LE cert yet — generating bootstrap self-signed for ${CERT_PRIMARY}"
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

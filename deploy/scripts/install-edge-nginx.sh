#!/usr/bin/env bash
# Install host nginx edge config for *-dew.fadosoft.com public surfaces.
# Run from repo root on the public host:
#   sudo bash deploy/scripts/install-edge-nginx.sh
# Then issue certs:
#   export CERTBOT_EMAIL=ops@fadosoft.com
#   sudo bash deploy/scripts/setup-certbot.sh

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONF_SRC="${ROOT}/deploy/nginx/dew-edge.conf"
CONF_DST=/etc/nginx/sites-available/dew-edge

if [[ "$(id -u)" -ne 0 ]]; then
  echo "error: run as root (sudo bash deploy/scripts/install-edge-nginx.sh)" >&2
  exit 1
fi

if [[ ! -f "${CONF_SRC}" ]]; then
  echo "error: missing ${CONF_SRC}" >&2
  exit 1
fi

apt-get update
apt-get install -y nginx certbot python3-certbot-nginx

install -m 644 "${CONF_SRC}" "${CONF_DST}"
ln -sf "${CONF_DST}" /etc/nginx/sites-enabled/dew-edge
rm -f /etc/nginx/sites-enabled/default

nginx -t
systemctl enable --now nginx
systemctl reload nginx

echo "Installed ${CONF_DST}"
echo "Next: open firewall 80/443, start backends on 127.0.0.1:8545/8081/8082,"
echo "      then: CERTBOT_EMAIL=you@domain sudo bash deploy/scripts/setup-certbot.sh"

#!/usr/bin/env bash
# Rollback for a failed edge start: stops the edge and lets TableBot's own Caddy publish 80/443
# again with its old certificate (still in tablebot_caddy_data). TutorHub stays offline.
#   bash /opt/tutorhub/deploy/edge/rollback-tablebot.sh
# The next TableBot deploy (push to its main) removes the ports again, so fix the edge first.
set -euo pipefail

EDGE_DIR="${EDGE_DIR:-/opt/edge}"
TABLEBOT_DIR="${TABLEBOT_DIR:-/opt/tablebot}"
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

step() { printf '\n==> %s\n' "$*"; }

step "Stopping the edge proxy"
if [[ -f "$EDGE_DIR/docker-compose.yml" ]]; then
  docker compose -p edge -f "$EDGE_DIR/docker-compose.yml" down
else
  echo "no edge installed in $EDGE_DIR"
fi

step "Publishing 80/443 from TableBot's own Caddy"
cd "$TABLEBOT_DIR"
docker compose -p tablebot --env-file .env \
  -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml -f "$SRC_DIR/tablebot-standalone.yml" \
  up -d --no-deps caddy
docker compose -p tablebot --env-file .env -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml ps caddy

site="$(grep -E '^SITE_ADDRESS=' .env | tail -n 1 | cut -d= -f2- | tr -d '"'"'"'')"
step "Check: curl -I https://$site"
sleep 3
curl -sSI --noproxy '*' --max-time 20 --resolve "$site:443:127.0.0.1" "https://$site" | head -n 1 || echo "not answering yet; see: docker compose -p tablebot logs caddy"

#!/usr/bin/env bash
# Deploys TutorHub on the server. Run as the normal user (not root); CI runs it over SSH.
#   bash /opt/tutorhub/deploy/deploy.sh            fetch origin/main, rebuild, restart, health-check
#   bash /opt/tutorhub/deploy/deploy.sh --no-pull  same, but deploy the current checkout (make prod-up)
# The web container publishes no ports: the shared edge proxy (deploy/edge) serves 80/443.
# Env overrides (for testing): APP_DIR (default /opt/tutorhub), COMPOSE_PROJECT (default tutorhub).
set -Eeuo pipefail

APP_DIR="${APP_DIR:-/opt/tutorhub}"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-tutorhub}"
HEALTH_TIMEOUT=90

step() { printf '\n==> %s\n' "$*"; }
fail() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }

compose() {
  docker compose -p "$COMPOSE_PROJECT" --env-file .env \
    -f deploy/docker-compose.yml -f deploy/docker-compose.prod.yml "$@"
}

# env_value KEY prints KEY's value from .env (last one wins, surrounding quotes removed).
env_value() {
  local v
  v="$(grep -E "^$1=" .env | tail -n 1 | cut -d= -f2-)" || true
  v="${v%\"}"; v="${v#\"}"; v="${v%\'}"; v="${v#\'}"
  printf '%s' "$v"
}

main() {
  local pull=1
  [[ "${1:-}" == "--no-pull" ]] && pull=0

  [[ "$(id -u)" -ne 0 ]] || fail "do not run as root; run it as your normal user (who is in the docker group)."
  cd "$APP_DIR" || fail "$APP_DIR does not exist. Run deploy/server-setup.sh first (see docs/deploy.md)."
  [[ -f .env ]] || fail "$APP_DIR/.env is missing. Create it: cp deploy/.env.production.example .env, then fill it in (docs/deploy.md, step 3)."
  docker info >/dev/null 2>&1 || fail "cannot talk to Docker. Log out and back in after server-setup.sh (docker group), or check 'sudo systemctl status docker'."

  local site
  site="$(env_value SITE_ADDRESS)"
  [[ -n "$site" ]] || fail "SITE_ADDRESS is empty in .env (it should be your TutorHub domain, e.g. tutorhub.example.kz)."
  grep -qE '^(POSTGRES_PASSWORD|BOT_TOKEN|BOT_USERNAME)=CHANGE_ME' .env && fail ".env still contains CHANGE_ME placeholders. Fill in POSTGRES_PASSWORD, BOT_TOKEN and BOT_USERNAME."
  [[ "$(env_value APP_ENV)" == "prod" ]] || fail "APP_ENV must be prod in .env (dev would accept fake logins)."

  if [[ "$pull" -eq 1 ]]; then
    step "Fetching the latest main from GitHub"
    git fetch --prune origin main
    git reset --hard origin/main
    # Continue with the freshly pulled version of this script.
    exec bash "$APP_DIR/deploy/deploy.sh" --no-pull
  fi

  step "Deploying commit $(git rev-parse --short HEAD): $(git log -1 --format=%s)"

  step "Docker network \"edge\" (shared with the edge proxy and other projects)"
  docker network create edge 2>/dev/null || true

  step "Building images and starting containers (first run takes a few minutes)"
  compose up -d --build --remove-orphans

  step "Removing unused images"
  docker image prune -f >/dev/null

  # Health is checked inside the web container, so it does not depend on the edge or DNS.
  step "Waiting up to ${HEALTH_TIMEOUT}s for http://tutorhub-web/api/health (inside the web container)"
  local deadline=$((SECONDS + HEALTH_TIMEOUT)) body=""
  while ((SECONDS < deadline)); do
    if body="$(compose exec -T caddy wget -qO- http://127.0.0.1/api/health 2>/dev/null)" && [[ "$body" == *'"status":"ok"'* ]]; then
      step "Deploy OK: $body"
      compose ps
      public_check "$site"
      return 0
    fi
    sleep 3
  done

  printf '\nERROR: /api/health did not return {"status":"ok"} within %ss.\n' "$HEALTH_TIMEOUT" >&2
  step "docker compose ps"
  compose ps -a || true
  for svc in migrate api caddy; do
    step "Last 80 log lines: $svc"
    compose logs --no-color --tail 80 "$svc" || true
  done
  printf '\nSee the troubleshooting table in docs/deploy.md.\n' >&2
  exit 1
}

# public_check reports whether the site answers through the edge proxy; it never fails the deploy.
public_check() {
  local site="$1" status
  [[ "$site" == :* ]] && return 0
  step "Public check through the edge proxy: https://$site/api/health"
  # --resolve hits this machine directly, so the check does not depend on external DNS.
  if status="$(curl -fsS --max-time 10 --resolve "${site}:443:127.0.0.1" "https://${site}/api/health" 2>&1)"; then
    echo "OK: $status"
  else
    echo "WARNING: not reachable through the edge yet ($status)."
    echo "Is the edge running and does /opt/edge/sites/$site.caddy exist? See docs/deploy.md (edge proxy)."
  fi
}

# Same line on purpose: bash never reads past it, even if git reset rewrote this file.
main "$@"; exit $?

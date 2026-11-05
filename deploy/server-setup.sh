#!/usr/bin/env bash
# One-time setup of TutorHub on an Ubuntu 24.04 VM that may already run TableBot. Safe to run again.
#   sudo bash server-setup.sh
# Installs Docker / ufw / swap / security updates only if missing, opens 80/443, clones the app to
# /opt/tutorhub, creates the shared "edge" Docker network and a nightly database backup.
# It never touches /opt/tablebot, TableBot's containers or the SSH server config.
set -Eeuo pipefail

REPO_URL="https://github.com/AlikoYe/tutorhub"
APP_DIR="/opt/tutorhub"

step() { printf '\n==> %s\n' "$*"; }
fail() { printf '\nERROR: %s\n' "$*" >&2; exit 1; }
trap 'fail "server-setup.sh stopped at line $LINENO. Fix the problem above and run it again (it is safe to re-run)."' ERR

[[ "$(id -u)" -eq 0 ]] || fail "run it with sudo: sudo bash $0"
APP_USER="${SUDO_USER:-}"
[[ -n "$APP_USER" && "$APP_USER" != "root" ]] || fail "run it with sudo from your normal user (e.g. azureuser), not from a root shell."
# shellcheck source=/dev/null
. /etc/os-release
[[ "${ID:-}" == "ubuntu" ]] || fail "this script supports Ubuntu only (found: ${PRETTY_NAME:-unknown})."
export DEBIAN_FRONTEND=noninteractive
# Ubuntu's needrestart would otherwise stop apt with an interactive "restart services?" dialog.
export NEEDRESTART_MODE=a
APT_OPTS=(-y -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold)

step "Base packages (already installed ones are skipped)"
apt-get update
apt-get "${APT_OPTS[@]}" install ca-certificates curl git make ufw cron unattended-upgrades

step "Docker"
if command -v docker >/dev/null && docker compose version >/dev/null 2>&1; then
  echo "already installed: $(docker --version)"
else
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${UBUNTU_CODENAME:-$VERSION_CODENAME} stable" \
    >/etc/apt/sources.list.d/docker.list
  apt-get update
  apt-get "${APT_OPTS[@]}" install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
fi
systemctl enable --now docker
docker compose version

step "Letting $APP_USER use Docker without sudo"
if id -nG "$APP_USER" | tr ' ' '\n' | grep -qx docker; then
  echo "$APP_USER is already in the docker group"
else
  usermod -aG docker "$APP_USER"
  echo "added; log out and back in for it to take effect"
fi

step "Swap"
if [[ -n "$(swapon --show --noheadings)" ]]; then
  echo "swap already enabled:"; swapon --show
else
  [[ -f /swapfile ]] || fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile >/dev/null
  swapon /swapfile
  grep -q '^/swapfile ' /etc/fstab || echo '/swapfile none swap sw 0 0' >>/etc/fstab
  echo "2 GB swapfile created"
fi

step "Firewall (ufw): SSH, 80 and 443 (rules that exist are kept)"
ufw allow OpenSSH >/dev/null
ufw allow 80/tcp >/dev/null
ufw allow 443/tcp >/dev/null
ufw allow 443/udp >/dev/null
if ufw status | grep -q "Status: active"; then
  echo "ufw already active"
else
  ufw --force enable
fi
ufw status verbose

step "Automatic security updates"
if [[ -f /etc/apt/apt.conf.d/20auto-upgrades ]]; then
  echo "already configured"
else
  cat >/etc/apt/apt.conf.d/20auto-upgrades <<'CONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
CONF
fi
systemctl enable --now unattended-upgrades

step "Shared Docker network \"edge\" (edge proxy + every project's web container)"
docker network create edge >/dev/null 2>&1 && echo "created" || echo "already exists"

step "App checkout in $APP_DIR (owned by $APP_USER)"
if [[ -d "$APP_DIR/.git" ]]; then
  chown -R "$APP_USER:$APP_USER" "$APP_DIR"
  sudo -u "$APP_USER" git -C "$APP_DIR" pull --ff-only
elif [[ -d "$APP_DIR" && -n "$(ls -A "$APP_DIR")" ]]; then
  fail "$APP_DIR exists but is not a git checkout. Move it away and run this script again."
else
  install -d -o "$APP_USER" -g "$APP_USER" "$APP_DIR"
  sudo -u "$APP_USER" git clone "$REPO_URL" "$APP_DIR"
fi
install -d -o "$APP_USER" -g "$APP_USER" "$APP_DIR/backups"

step "Nightly database backup at 03:30 (server time) into $APP_DIR/backups, keeping 7"
# 03:30 so it does not run at the same time as TableBot's 03:00 backup.
cat >/etc/cron.d/tutorhub-backup <<CRON
# Installed by $APP_DIR/deploy/server-setup.sh
30 3 * * * $APP_USER bash $APP_DIR/deploy/backup.sh >>$APP_DIR/backups/backup.log 2>&1
CRON
chmod 644 /etc/cron.d/tutorhub-backup
systemctl enable --now cron

trap - ERR
cat <<NEXT

==> Server setup finished. /opt/tablebot was not touched.

Next steps (docs/deploy.md has the details):
  1. If you were just added to the docker group: log out and back in.
  2. Create the config:   cd $APP_DIR && cp deploy/.env.production.example .env && nano .env
  3. Prepare the edge proxy without starting it (cutover step a in docs/deploy.md).
NEXT

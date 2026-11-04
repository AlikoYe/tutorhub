#!/usr/bin/env bash
# One-time setup of a fresh Ubuntu 24.04 VM for TutorHub. Safe to run again.
#   sudo bash server-setup.sh
# Installs Docker, a swapfile, the firewall, automatic security updates, the app checkout
# in /opt/tutorhub and a nightly database backup. It never touches the SSH server config.
set -Eeuo pipefail

REPO_URL="https://github.com/AlikoYe/TutorHub"
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

step "Updating packages (can take a few minutes)"
apt-get update
apt-get "${APT_OPTS[@]}" upgrade
apt-get "${APT_OPTS[@]}" install ca-certificates curl git make ufw cron unattended-upgrades

step "Installing Docker Engine and the compose plugin from Docker's apt repository"
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${UBUNTU_CODENAME:-$VERSION_CODENAME} stable" \
  >/etc/apt/sources.list.d/docker.list
apt-get update
apt-get "${APT_OPTS[@]}" install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
systemctl enable --now docker
docker compose version

step "Letting $APP_USER use Docker without sudo"
usermod -aG docker "$APP_USER"

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

step "Firewall (ufw): SSH first, then web"
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 443/udp
ufw --force enable
ufw status verbose

step "Automatic security updates"
cat >/etc/apt/apt.conf.d/20auto-upgrades <<'CONF'
APT::Periodic::Update-Package-Lists "1";
APT::Periodic::Unattended-Upgrade "1";
CONF
systemctl enable --now unattended-upgrades

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

step "Nightly database backup at 03:00 (server time) into $APP_DIR/backups, keeping 7"
cat >/etc/cron.d/tutorhub-backup <<CRON
# Installed by $APP_DIR/deploy/server-setup.sh
0 3 * * * $APP_USER bash $APP_DIR/deploy/backup.sh >>$APP_DIR/backups/backup.log 2>&1
CRON
chmod 644 /etc/cron.d/tutorhub-backup
systemctl enable --now cron

trap - ERR
cat <<NEXT

==> Server setup finished.

Next steps (docs/deploy.md has the details):
  1. Log out and back in, so $APP_USER can use docker:   exit   (then ssh in again)
  2. Create the config:   cd $APP_DIR && cp deploy/.env.production.example .env && nano .env
  3. First deploy:        bash $APP_DIR/deploy/deploy.sh
NEXT

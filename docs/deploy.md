# Deploying TutorHub (next to TableBot, on the same VM)

TutorHub runs on the **same Azure VM** as TableBot
(`tablebot-g30.indiasouthcentral.cloudapp.azure.com`). Only one program can own ports
80/443, so both projects sit behind one shared **edge proxy**:

```
internet ──80/443──▶ edge (Caddy, project "edge", /opt/edge, gets all HTTPS certificates)
                       ├─ tablebot host ──▶ tablebot-web:80  (TableBot's web container)
                       └─ tutorhub host ──▶ tutorhub-web:80  (TutorHub's web container)
                                             ├─ /api/*, /t/*, /c/*, sitemap ─▶ api:8080
                                             └─ everything else ─▶ the Vue app
```

| Project | Directory | Compose project | Containers |
|---|---|---|---|
| Edge proxy | `/opt/edge` | `edge` | `edge` (Caddy, the only one publishing 80/443) |
| TableBot | `/opt/tablebot` | `tablebot` | `postgres`, `migrate`, `api`, `caddy` (alias `tablebot-web`) |
| TutorHub | `/opt/tutorhub` | `tutorhub` | `postgres`, `migrate`, `api`, `caddy` (alias `tutorhub-web`) |

The web containers serve plain HTTP on `:80` and join the external Docker network `edge`
with a unique alias (both services are called `caddy` inside their own projects, so the
edge always proxies to the aliases). Each project keeps its own Postgres; no database port
is published. Edge files: `deploy/edge/` in this repo. They are installed to `/opt/edge` by
`deploy/edge/install.sh`.

Replace these placeholders below:

- `<tutorhub-host>`: TutorHub's domain, e.g. `tutorhub.example.kz` (see step 1)
- `<your-telegram-id>`: `888659722`
- `<vm-ip>`: the VM's public IP (Azure portal → VM → Overview → Public IP address)

---

## 1. A domain for TutorHub

The Azure DNS label already belongs to TableBot, so TutorHub needs its own hostname.

**Option A, your own domain (recommended).** At your DNS provider, add an **A record**:

| Type | Name | Value | TTL |
|---|---|---|---|
| A | `tutorhub` (or `@` for the bare domain) | `<vm-ip>` | 300 |

Check it on your Mac (it can take a few minutes):

```bash
dig +short <tutorhub-host>
```

✅ It prints `<vm-ip>`.

**Option B, no domain yet.** Use `tutorhub.<vm-ip-with-dashes>.sslip.io`, for example
`tutorhub.20-1-2-3.sslip.io`. It already resolves to that IP, and Let's Encrypt issues
certificates for it. Switch to a real domain later by re-running step 4 with the new host
and updating `.env` (step 3).

## 2. The production bot (BotFather)

Keep your laptop's dev bot. Create a **second** bot for production (one token, one place:
two programs polling one token steal each other's updates).

1. In Telegram, open [@BotFather] and send `/newbot`. Name it `TutorHub`, username e.g. `tutorhub_kz_bot`.
2. Copy the token. It goes into `BOT_TOKEN` in step 3. Never paste it anywhere else.
3. Send `/setdescription` and `/setabouttext` with a short Russian description (optional).

The **menu button** ("TutorHub", which opens the Mini App) is set by the API itself on every
start from `WEBAPP_URL` (step 7 shows how to check it). No BotFather step is needed for it.

[@BotFather]: https://t.me/BotFather

## 3. Server setup and config

SSH into the VM:

```bash
ssh azureuser@tablebot-g30.indiasouthcentral.cloudapp.azure.com
```

Run the TutorHub setup. It installs only what is missing (Docker, ufw rules and swap already
exist because of TableBot), clones the app to `/opt/tutorhub`, creates the `edge` network
and a nightly backup at 03:30. It never touches `/opt/tablebot`.

```bash
curl -fsSL https://raw.githubusercontent.com/AlikoYe/tutorhub/main/deploy/server-setup.sh -o tutorhub-setup.sh
```

```bash
sudo bash tutorhub-setup.sh
```

✅ It ends with `Server setup finished. /opt/tablebot was not touched.`
If it says you were added to the docker group, log out (`exit`) and SSH in again.

Create the config:

```bash
cd /opt/tutorhub && cp deploy/.env.production.example .env && chmod 600 .env && nano .env
```

Fill in at least:

| Variable | Value |
|---|---|
| `POSTGRES_PASSWORD` | output of `openssl rand -hex 24` |
| `BOT_TOKEN` | the token from step 2 |
| `BOT_USERNAME` | the bot's username without `@` |
| `WEBAPP_URL` | `https://<tutorhub-host>` |
| `SITE_URL` | `https://<tutorhub-host>` |
| `SITE_ADDRESS` | `<tutorhub-host>` (no `https://`) |
| `ADMIN_TELEGRAM_IDS` | `888659722` |

✅ `grep CHANGE_ME .env` prints nothing.

> Until `main` contains the deploy files (this PR merged), use the branch:
> `cd /opt/tutorhub && git fetch origin m6-deploy && git checkout m6-deploy`.
> Merge to `main` before step 7, because automatic deploys reset to `origin/main`.

## 4. CUTOVER: move 80/443 from TableBot to the edge

Do these steps in one sitting. TableBot is **offline between b) and c)**, normally under a minute.

### a) Prepare the edge without starting it

```bash
TABLEBOT_HOST=tablebot-g30.indiasouthcentral.cloudapp.azure.com TUTORHUB_HOST=<tutorhub-host> bash /opt/tutorhub/deploy/edge/install.sh --no-start
```

✅ Expected output (abridged):

```
==> Docker network "edge"
already exists
==> Stack files in /opt/edge
copied docker-compose.yml and Caddyfile
==> Site files in /opt/edge/sites
tablebot-g30.indiasouthcentral.cloudapp.azure.com -> tablebot-web:80
<tutorhub-host> -> tutorhub-web:80
==> Validating the Caddy config
Valid configuration
config is valid
==> Not starting the edge (--no-start). ...
```

The hosts are saved in `/opt/edge/edge.env`, so later runs need no variables.

### b) Merge the TableBot edge PR

Merge **https://github.com/AlikoYe/TableBot/pull/5** ("Serve TableBot behind the shared edge
proxy", branch `edge-proxy`) on GitHub. Its deploy workflow runs `deploy.sh` on the server. TableBot's web
container stops publishing 80/443 and joins the `edge` network as `tablebot-web`.
**TableBot is unreachable from the internet from now until c).**

Watch the GitHub Actions run (TableBot → Actions → deploy), or on the server:

```bash
docker ps --format '{{.Names}}\t{{.Ports}}' | grep tablebot
```

✅ The deploy log ends with `Deploy OK: http://tablebot-web/api/health -> {"status":"ok"}`
followed by `WARNING: not reachable through the edge proxy yet` (expected: the edge is not
running yet). `tablebot-caddy-1` shows `80/tcp, 443/tcp...` **without** `0.0.0.0:` (nothing published).

### c) Start the edge immediately

```bash
bash /opt/tutorhub/deploy/edge/install.sh --copy-certs
```

`--copy-certs` copies TableBot's existing certificate into the edge, so no new certificate is
needed for it.

✅ Expected output ends with:

```
==> Copying TableBot certificates into edge_data
copied
==> Starting the edge
 Container edge-edge-1  Started
...
tablebot-g30.indiasouthcentral.cloudapp.azure.com: HTTP/2 200
<tutorhub-host>: HTTP/2 502
```

`502` for TutorHub is expected until step d). Check TableBot from your Mac:

```bash
curl -sI https://tablebot-g30.indiasouthcentral.cloudapp.azure.com | head -1
```

✅ `HTTP/2 200`. TableBot is back. If not, see e) right away.

### d) First TutorHub deploy

```bash
cd /opt/tutorhub && bash deploy/deploy.sh --no-pull
```

✅ It ends with:

```
==> Deploy OK: {"status":"ok"}
...
==> Public check through the edge proxy: https://<tutorhub-host>/api/health
OK: {"status":"ok"}
```

The first certificate for `<tutorhub-host>` can take a minute. If the public check warns,
wait and run `curl -sI https://<tutorhub-host> | head -1` again.

Optional demo data (8 fake Almaty teachers; hide them later with `make prod-unseed`):

```bash
cd /opt/tutorhub && make prod-seed ADMIN=888659722
```

✅ Ends with `COMMIT`.

Health checks from your Mac:

```bash
curl -s https://<tutorhub-host>/api/health
```

```bash
curl -sI https://<tutorhub-host>/c/almaty | head -1
```

```bash
curl -s https://tablebot-g30.indiasouthcentral.cloudapp.azure.com/api/health
```

✅ `{"status":"ok"}`, `HTTP/2 200`, `{"status":"ok"}`.

Menu button: open the production bot in Telegram and send `/start`.
✅ The bot answers with **📱 Открыть TutorHub**, and the button next to the message field
says **TutorHub** (restart Telegram if you still see the old one). The API sets it on start
from `WEBAPP_URL`; `make prod-logs` shows `telegram bot started (long polling)`.

Admin: in the app, the **Админ** tab appears for `888659722`.

### e) Rollback if c) fails

One command puts 80/443 back on TableBot's own Caddy, with its old certificate, and stops
the edge (TutorHub goes offline, TableBot comes back):

```bash
bash /opt/tutorhub/deploy/edge/rollback-tablebot.sh
```

✅ It ends with `Check: curl -I https://tablebot-g30...` followed by `HTTP/2 200`.

The next TableBot deploy (any push to TableBot's `main`) removes the ports again. Fix the
edge (troubleshooting below), then run c) again; or revert the TableBot PR on GitHub, which
redeploys TableBot in its old standalone form.

## 5. Automatic deploys from GitHub (TutorHub)

GitHub Actions deploys every push to `main` by SSH-ing in and running `deploy.sh`. TutorHub
uses its **own** key, which can only run TutorHub's deploy script.

**5a. Create the key on your Mac:**

```bash
ssh-keygen -t ed25519 -N "" -C "tutorhub-deploy" -f ~/.ssh/tutorhub_deploy
```

**5b. Allow it on the server, restricted to the deploy script:**

```bash
echo "restrict,command=\"bash /opt/tutorhub/deploy/deploy.sh\" $(cat ~/.ssh/tutorhub_deploy.pub)" | ssh azureuser@tablebot-g30.indiasouthcentral.cloudapp.azure.com 'cat >> ~/.ssh/authorized_keys'
```

**5c. Test it (runs a real deploy):**

```bash
ssh -i ~/.ssh/tutorhub_deploy azureuser@tablebot-g30.indiasouthcentral.cloudapp.azure.com
```

✅ You see the deploy output ending in `Deploy OK`, then the connection closes.

**5d. Add the repository secrets** (github.com/AlikoYe/tutorhub → Settings → Secrets and
variables → Actions → New repository secret):

| Secret | Value |
|---|---|
| `DEPLOY_HOST` | `tablebot-g30.indiasouthcentral.cloudapp.azure.com` |
| `DEPLOY_USER` | `azureuser` |
| `DEPLOY_SSH_KEY` | the whole output of `cat ~/.ssh/tutorhub_deploy` |
| `DEPLOY_KNOWN_HOSTS` | the output of `ssh-keyscan tablebot-g30.indiasouthcentral.cloudapp.azure.com` |

✅ The next push to `main` shows a green **deploy** run under Actions. Without `DEPLOY_HOST`,
the workflow skips deploying with a notice.

## Everyday commands (on the server)

| What | Command |
|---|---|
| TutorHub status | `cd /opt/tutorhub && make prod-ps` |
| TutorHub logs | `cd /opt/tutorhub && make prod-logs` |
| Redeploy the current checkout | `cd /opt/tutorhub && make prod-up` |
| Database shell | `cd /opt/tutorhub && make prod-psql` |
| Backup now | `cd /opt/tutorhub && make prod-backup` |
| Restore a backup | `cd /opt/tutorhub && make prod-restore FILE=backups/tutorhub-YYYYMMDD-HHMMSS.sql.gz` |
| Edge logs | `cd /opt/tutorhub && make edge-logs` |
| Edge status | `docker compose -p edge -f /opt/edge/docker-compose.yml ps` |
| Change a host / add a site | re-run step 4a's command with the new host, then `bash /opt/tutorhub/deploy/edge/install.sh` |
| Who holds 80/443 | `docker ps --format '{{.Names}}\t{{.Ports}}' \| grep -E ':(80\|443)->'` |

Backups: `/opt/tutorhub/backups`, nightly at 03:30 (TableBot's run at 03:00), the newest 7 kept.
Uploaded photos and documents live in the `tutorhub_uploads` volume, which the database
backup does not include. Copy it with:

```bash
docker run --rm -v tutorhub_uploads:/data:ro -v /opt/tutorhub/backups:/out alpine:3 tar czf /out/uploads-$(date +%F).tgz -C /data .
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| Edge does not start: `port is already allocated` / `address already in use` | Another Caddy still publishes 80/443 (TableBot before its edge PR is deployed) | `docker ps --format '{{.Names}}\t{{.Ports}}' \| grep -E ':(80\|443)->'`. Deploy the TableBot PR (step 4b), then run 4c again. |
| `502 Bad Gateway` for a host | Its web container is down or not on the `edge` network | `docker network inspect edge --format '{{range .Containers}}{{.Name}} {{end}}'` must list `tablebot-caddy-1` and `tutorhub-caddy-1`. Redeploy that project (`bash deploy/deploy.sh --no-pull`). |
| Certificate errors / `obtaining certificate` in edge logs | DNS does not point to the VM yet, or 80/443 are blocked | `dig +short <host>` must print `<vm-ip>`. Azure Networking must allow inbound 80 and 443. `sudo ufw status` must list 80/tcp and 443/tcp. Then `docker compose -p edge -f /opt/edge/docker-compose.yml restart`. |
| `deploy.sh`: `WARNING: not reachable through the edge proxy yet` | Edge not running, or no site file for this host | `ls /opt/edge/sites`, then run step 4a's command with the host, then `bash /opt/tutorhub/deploy/edge/install.sh`. |
| `deploy.sh`: `.env still contains CHANGE_ME` / `APP_ENV must be prod` | Config not filled in | Step 3. |
| Mini App says "Не удалось войти" | Opened outside Telegram, or `BOT_TOKEN` is not the bot whose menu opened the app | Open the app from the production bot. Check `BOT_TOKEN` in `.env`, then `make prod-up`. |
| Bot ignores `/start` or buttons at random | The same token runs somewhere else too (laptop) | One token, one place: give the laptop the dev bot's token. |
| A teacher gets no lesson requests in the bot | The teacher never pressed **Start** in the bot | Send `/start` to the bot. `telegram send failed ... chat not found` in the api log means exactly this. |
| Wrong client IPs in logs | The inner Caddy does not trust the edge | Both inner Caddyfiles have `trusted_proxies static private_ranges`; redeploy that project. |
| GitHub deploy: `Permission denied (publickey)` | `DEPLOY_SSH_KEY` or the `authorized_keys` line is wrong | Redo 5b and 5d; test with 5c. |

### Roll back TutorHub to the previous version

```bash
cd /opt/tutorhub && git log --oneline -5
```

```bash
git reset --hard <commit> && bash deploy/deploy.sh --no-pull
```

The next push to `main` deploys `main` again; revert the bad commit on GitHub to keep it away.

# Contabo VPS — deployment setup and operations

This is the record of how RVPay is actually deployed on our Contabo VPS, and
the runbook for operating it. `README.md` in this folder describes the
generic Contabo stack; this file covers the specifics of our server, which
runs two RVPay environments (testing and production) and also hosts other
live sites.

First brought up: 2026-10-07, from the `contabo-migration` branch.

**The Contabo setup is independent of the AWS and Render deployments.** It
uses its own branches (`contabo/testing`, `contabo/production`), its own
GitHub Environments (`contabo-testing`, `contabo-production`) and temporary
hostnames. Nothing here touches `main`, `release/**`, `cd/environments.yaml`
or the AWS/Render workflows. Production is served on `rvpay.co`, which was
never configured in AWS. Testing has been served on `rvpay.xyz` since
2026-10-09, when its `api` and `admindashboard` records were moved off the
AWS load balancer, which took AWS testing off those hostnames (section 10).

---

## 1. At a glance

| | |
|---|---|
| Provider | Contabo VPS |
| Public IP | `75.119.147.69` |
| OS / arch | Ubuntu 24.04 LTS, **x86_64** (images build as `linux/amd64`) |
| Size | 4 vCPU, 7.8 GB RAM, 96 GB disk, 4 GB swap file |
| SSH | `root@75.119.147.69`, key-based |

| | Testing | Production |
|---|---|---|
| Dashboard | **https://admindashboard.rvpay.xyz** (also `admindashboard.testing.75-119-147-69.sslip.io`) | **https://admindashboard.rvpay.co** (also `admindashboard.production.75-119-147-69.sslip.io`) |
| API | **https://api.rvpay.xyz** (also `api.testing.75-119-147-69.sslip.io`) | **https://api.rvpay.co** (also `api.production.75-119-147-69.sslip.io`) |
| Dashboard environment | picked automatically from the hostname (**Testing** on `rvpay.xyz`, **Contabo Testing** on sslip.io) | picked automatically (**Production** on `rvpay.co`, **Contabo Production** on sslip.io) |
| Checkout | `/opt/rvpay-testing` | `/opt/rvpay-production` |
| Secrets | `/opt/rvpay-testing/.env` | `/opt/rvpay-production/.env` |
| Compose project | `rvpay-testing` | `rvpay-production` |
| Host ports (clients / transactions / dashboard) | 8080 / 8081 / 3002 | 8090 / 8091 / 3003 |
| nginx site | `/etc/nginx/sites-available/rvpay-testing` | `/etc/nginx/sites-available/rvpay-production` |
| Boot service | `rvpay@testing` | `rvpay@production` |
| Deploy branch | `contabo/testing` | `contabo/production` |
| GitHub Environment | `contabo-testing` | `contabo-production` |

The `*.sslip.io` hostnames are temporary: sslip.io resolves any name that
contains `<ip-with-dashes>.sslip.io` to that IP, so we get real hostnames and
real Let's Encrypt certificates without touching DNS.

### Shared server — read this first

The VPS is **not** dedicated to RVPay. It also runs two unrelated live sites:

| Site | Served by | Port |
|---|---|---|
| theboldagenda.org | PM2 (`theboldagenda`), `/var/www/theboldagenda` | 3000 |
| my.citscm.com | PM2 (`attendee-app`), `/var/www/attendee-app` | 3001 |

Both sit behind the **host** nginx on 80/443, with certbot certificates.
Rules that follow from that:

- Never publish a Docker port on 80, 443, 3000 or 3001.
- Never publish a Docker port on `0.0.0.0`. Docker-published ports bypass
  `ufw`, so a public mapping is reachable even though the firewall says deny.
  Every RVPay port is bound to `127.0.0.1`.
- Always run `nginx -t` before `systemctl reload nginx`. A broken nginx
  config takes every site on the box down.
- Heavy builds run under `nice` so they don't starve the other apps.

Testing and production share this machine too. A runaway testing deploy or
a reboot affects production. Move production to its own VPS once it has real
traffic. Nothing in this setup changes except `CONTABO_HOST`.

---

## 2. Architecture

```
                              Internet
                                 │  80 / 443 (ufw: only 22, 80, 443 open)
                                 ▼
                 ┌────────────────────────────────┐
                 │      host nginx + certbot      │──► theboldagenda.org → :3000 (PM2)
                 │                                │──► my.citscm.com     → :3001 (PM2)
                 └────────────────────────────────┘
                     │                         │
     *.testing.75-119-147-69.sslip.io   *.production.75-119-147-69.sslip.io
                     │                         │
          127.0.0.1:8080/8081/3002    127.0.0.1:8090/8091/3003
  ┌──── Compose project rvpay-testing ──┐ ┌── Compose project rvpay-production ──┐
  │ clients ◄─gRPC─► transactions       │ │ clients ◄─gRPC─► transactions        │
  │     └──► postgres ◄──┘  dashboard   │ │     └──► postgres ◄──┘  dashboard    │
  │  own network, own volume, own .env  │ │  own network, own volume, own .env   │
  └─────────────────────────────────────┘ └──────────────────────────────────────┘
```

### Database separation

Each environment has a **completely separate database**: not two databases
on one server, but a separate Postgres server each.

| | Testing | Production |
|---|---|---|
| Postgres container | `rvpay-testing-postgres-1` | `rvpay-production-postgres-1` |
| Data volume | `rvpay-testing_postgres-data` | `rvpay-production_postgres-data` |
| Private network | `rvpay-testing_default` | `rvpay-production_default` |
| `DB_PASSWORD` | its own (in `/opt/rvpay-testing/.env`) | a different one (in `/opt/rvpay-production/.env`) |

Each Postgres holds that environment's own `clients` and `transactions`
databases. The separation works on four layers:

1. **Separate servers:** each environment runs its own Postgres container.
2. **Separate storage:** wiping or restoring one volume can't affect the
   other.
3. **Separate networks:** production's Postgres can't even be resolved
   from testing's network. Checked on 2026-10-07 with
   `docker run --rm --network rvpay-testing_default postgres:16-alpine getent hosts rvpay-production-postgres-1`,
   which finds nothing.
4. **Separate passwords:** testing's credentials don't work on production.

Testing can be reset or reloaded freely without any risk to production.

API routing (same prefixes in every `nginx*.conf`):

| Path prefix | Goes to |
|---|---|
| `/v1/public/auth`, `/v1/public/clients`, `/v1/public/integrations`, `/v1/public/platforms`, `/oauth/callback`, `/payments/custom-provider`, `/webhooks/highlevel` | clients |
| `/v1/public/merchants`, `/v1/public/customers`, `/v1/public/deposits`, `/v1/public/payments`, `/v1/public/payouts`, `/v1/public/transactions` | transactions |
| anything else on `api.*` | 404 |
| everything on `admindashboard.*` | admindashboard |

Inside each container the services use the ports coded in the repo (HTTP
8080, gRPC 50051, dashboard 3000). Only the host side of the mapping differs
per environment. Raw gRPC between clients and transactions never leaves the
Compose network.

---

## 3. Files involved

| File | Purpose |
|---|---|
| `docker-compose.contabo.yml` | Base stack: postgres, one-shot migration jobs, clients, transactions, admindashboard, bundled nginx |
| `docker-compose.contabo.hostnginx.yml` | **Override used on our server.** Disables the bundled nginx, and publishes the gateways on `127.0.0.1` at the ports set in `.env` |
| `deploy/contabo/nginx-host-site.testing.conf`, `nginx-host-site.production.conf` | Host nginx sites, one per environment |
| `deploy/contabo/nginx.conf` | Config for the bundled nginx container (not used on this server) |
| `deploy/contabo/postgres-init/001-create-service-databases.sh` | Creates the `clients` and `transactions` databases on first Postgres start |
| `deploy/contabo/systemd/rvpay@.service` | Per-environment boot unit (`rvpay@testing`, `rvpay@production`) |
| `deploy/contabo/systemd/rvpay-go.service` | Original single-stack unit; superseded on this server |
| `deploy/contabo/.env.example` | Template for each environment's `.env` |
| `admindashboard/lib/environments.ts` | Dashboard backend URLs, including `contabo-testing` and `contabo-production` |
| `.github/workflows/deploy-contabo.yml` | CI: test, build images once per commit, deploy to one environment |

Every `docker compose` command on the server names the project and both files:

```bash
ENV=testing   # or production
cd /opt/rvpay-$ENV
C="docker compose -p rvpay-$ENV -f docker-compose.contabo.yml -f docker-compose.contabo.hostnginx.yml"
```

The rest of this document uses `$ENV` and `$C` as shorthand. **Forgetting
`-p` runs commands against the wrong (or a new) project.**

---

## 4. Configuration and secrets

There are two places configuration lives:

| Where | Contains | In git? |
|---|---|---|
| `docker-compose.contabo.yml`, per-service `environment:` | Non-secret, per-service values: `DB_HOST=postgres`, `DB_PORT`, `DB_NAME` (`clients` / `transactions`), `DB_TLS_DISABLED`, `LISTEN_PORT=50051`, `HTTP_PORT=8080`, `MIGRATION_PATH`, `RUN_MIGRATIONS=false`, `CLIENTS_GRPC_ADDR`, `TRANSACTIONS_GRPC_ADDR` | Yes |
| `/opt/rvpay-<env>/.env` (one per environment, shared by both services via `env_file`) | Host ports, secrets and integration settings: `*_HOST_PORT`, `DB_USER`, `DB_PASSWORD`, `HIGHLEVEL_*`, `PUBLIC_BASE_URL`, `PAWAPAY_*`, `HTTP_CORS_ALLOWED_ORIGINS`, `LOG_LEVEL` | **No** (`**/.env` is gitignored) |

How it works:

- Values in a service's `environment:` block override the same name from
  `.env`. That's how both services share one `.env` and still get different
  `DB_NAME`s and migration paths.
- `DB_USER` and `DB_PASSWORD` are shared on purpose: one Postgres role
  (`rvpay`) owns both databases. Each environment has its own password.
- If a secret ever needs a **different value per service**, give it a
  different name (e.g. `CLIENTS_X` / `TRANSACTIONS_X`). Don't put it in the
  compose file.
- Multi-line values (the HighLevel webhook public key) **must be
  double-quoted**, or Compose can't parse them.
- The clients service reads `HIGHLEVEL_REDIRECT_URL` (not `..._URI`). The
  service won't start if a `required` setting is missing.
- `HTTP_CORS_ALLOWED_ORIGINS` must include that environment's dashboard
  origin, e.g. `https://admindashboard.testing.75-119-147-69.sslip.io`.

Deploys never touch the secrets in `.env`. The only lines a CI deploy
writes are the image pins, `CLIENTS_IMAGE`, `TRANSACTIONS_IMAGE` and
`ADMINDASHBOARD_IMAGE`, set to the GHCR images of the deployed commit. That
way a later manual `$C up -d` keeps running the deployed images instead of
falling back to the `rvpay/<service>:local` defaults. After editing `.env`,
apply it with `$C up -d --no-build`, which recreates only the containers
whose config changed.

### Rotating the database password

The services connect as the `rvpay` Postgres role. To change its password
in one environment (about 2 minutes, plus a short restart of the apps):

```bash
NEW=$(openssl rand -hex 24)
$C exec -T postgres sh -c "psql -v ON_ERROR_STOP=1 -U \"\$POSTGRES_USER\" -d postgres -c \"ALTER ROLE \\\"\$POSTGRES_USER\\\" PASSWORD '$NEW'\""
sed -i "s/^DB_PASSWORD=.*/DB_PASSWORD=$NEW/" .env; unset NEW
$C up -d --no-build      # recreates the containers with the new password
```

Both environments were rotated on 2026-10-08, after the services had been
found printing the connection string (password included) to their logs.
That logging was removed in commit `0e28e78`.

---

## 5. Deploying

### 5.1 How it works

This mirrors the AWS flow: **build once, promote the same images.**

1. Push a commit to **`contabo/testing`**. `.github/workflows/deploy-contabo.yml`
   runs the Go tests, builds the clients, transactions and admindashboard
   images, tags them with the commit SHA, pushes them to GHCR, and deploys
   them to testing.
2. Check it on testing.
3. Push **the same commit** to **`contabo/production`**. The build step finds
   the images for that SHA already in GHCR and skips the rebuild, so
   production runs exactly what was tested.

```bash
# deploy the current branch's HEAD to testing
git push origin HEAD:contabo/testing

# promote what's on testing to production
git push origin origin/contabo/testing:contabo/production
```

Each deploy SSHes into the server and runs these steps in `/opt/rvpay-<env>`:

1. Log the server in to GHCR with the job's short-lived token.
2. `git checkout --detach <sha>`, so the compose files match the images.
3. `pull`, then `up -d --no-build`. The migration jobs run first,
   automatically.
4. Log out of GHCR, then run the health checks.

Deploys to the same environment are queued, never run in parallel.

If `contabo-production` has required reviewers, a production run pauses
before deploying. To approve it, go to **Actions → Deploy to Contabo → (the
run) → Review deployments → Approve and deploy**.

The first CI deploy to testing (commit `76cdd02`, 2026-10-07) reached the
server about 3 minutes after the push.

### 5.1a Deploys never clear the database

A deploy only replaces the **app** containers. Data is kept:

- Data lives in the environment's Docker **volume**
  (`rvpay-<env>_postgres-data`), not in a container. Containers are
  replaced on every deploy; the volume stays.
- The workflow only runs `pull` and `up -d`. It never runs `down`, `-v`,
  `docker volume rm` or `prune`.
- The Postgres image (`postgres:16-alpine`) doesn't change between deploys,
  so Compose usually doesn't even restart Postgres.
- The migration jobs apply only new `*.up.sql` files and skip ones already
  applied. `*.down.sql` files are never run by a deploy.
- `git checkout` doesn't touch `.env` or the data. Both are outside git.

Verified on the first CI deploy: testing's admin user survived, and Postgres
stayed up throughout, while clients, transactions and the dashboard were
replaced.

What **can** lose or hide data (all manual, and none of it is in the
pipeline):

| Action | Effect |
|---|---|
| `$C down -v` | **Deletes the volume.** Never add `-v`. |
| `docker volume rm ...`, `docker system prune --volumes` | Deletes volumes |
| A new migration that drops a table or column | Changes the data, but only because that code says so. Review migrations in PRs. |
| Running compose **without `-p rvpay-<env>`**, or from another folder | Deletes nothing, but starts a *new, empty* database, so the data looks gone |

These branch names don't match `main` or `release/**`, so they never trigger
the AWS (`build_and_publish.yaml`, `ci.yaml`) or Render pipelines.

`workflow_dispatch` (the "Run workflow" button) only appears once this
workflow file is on the default branch. Until then, deploy by pushing to the
branches.

### 5.2 GitHub setup (one-time)

In **Settings → Environments**, create `contabo-testing` and
`contabo-production`. Don't reuse the AWS `testing`/`production`
environments. Give each these secrets:

| Secret | Value |
|---|---|
| `CONTABO_HOST` | `75.119.147.69` |
| `CONTABO_USER` | `root` |
| `CONTABO_SSH_PRIVATE_KEY` | Private half of the dedicated `github-actions-rvpay-deploy` key (its public half is in `/root/.ssh/authorized_keys` on the server) |

On `contabo-production`, add **Required reviewers** so production deploys
wait for approval.

Images go to `ghcr.io/mountainhubtech/rvpay-go-{clients,transactions,admindashboard}`
using the built-in `GITHUB_TOKEN`. No extra registry secret is needed.

### 5.3 Manual deploy (fallback)

```bash
ENV=testing; cd /opt/rvpay-$ENV
C="docker compose -p rvpay-$ENV -f docker-compose.contabo.yml -f docker-compose.contabo.hostnginx.yml"
git fetch && git checkout --detach origin/contabo/$ENV
sed -i '/^\(CLIENTS\|TRANSACTIONS\|ADMINDASHBOARD\)_IMAGE=/d' .env   # drop the CI image pins
nice -n 10 $C up -d --build
```

This builds locally as `rvpay/<service>:local` images, instead of pulling
from GHCR. Removing the image pins is required: otherwise Compose keeps
using the last CI-deployed images. The next CI deploy writes the pins back.

---

## 6. Setting up an environment from scratch

All as `root` on the VPS. Sections 6.1–6.2 are once per server; 6.3 onwards
are once per environment.

### 6.1 SSH access

From an operator PC, key-based login with an alias in `~/.ssh/config`:

```
Host rvpay-vps
    HostName 75.119.147.69
    User root
    IdentityFile ~/.ssh/rvpay_vps
```

The server has its own key (`/root/.ssh/id_ed25519`), registered on GitHub,
with read access to `MountainHubTech/rvpay-go`.

### 6.2 Swap and Docker (once per server)

```bash
# 4 GB swap
fallocate -l 4G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile
echo '/swapfile none swap sw 0 0' >> /etc/fstab
echo 'vm.swappiness=10' > /etc/sysctl.d/99-swappiness.conf

# Docker Engine + Compose plugin from Docker's apt repo
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo $VERSION_CODENAME) stable" \
  > /etc/apt/sources.list.d/docker.list
apt-get update && apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Container log rotation
cat > /etc/docker/daemon.json <<'EOF'
{ "log-driver": "json-file", "log-opts": { "max-size": "10m", "max-file": "3" } }
EOF
systemctl restart docker

# Boot unit template
cp deploy/contabo/systemd/rvpay@.service /etc/systemd/system/ && systemctl daemon-reload
```

### 6.3 Checkout and secrets

```bash
ENV=testing
git clone git@github.com:MountainHubTech/rvpay-go.git /opt/rvpay-$ENV
cd /opt/rvpay-$ENV && git checkout --detach origin/contabo/$ENV
cp deploy/contabo/.env.example .env && chmod 600 .env
# fill in every value, and set this environment's *_HOST_PORT values (section 1)
```

### 6.4 Start and enable on boot

```bash
nice -n 10 $C up -d --build      # or let the first CI deploy do it
systemctl enable rvpay@$ENV
```

Start-up order: postgres (healthy) → `migration-clients` /
`migration-transactions` (one-shot, must exit 0) → clients, transactions,
admindashboard.

### 6.5 Host nginx and TLS

```bash
cp deploy/contabo/nginx-host-site.$ENV.conf /etc/nginx/sites-available/rvpay-$ENV
ln -s /etc/nginx/sites-available/rvpay-$ENV /etc/nginx/sites-enabled/rvpay-$ENV
nginx -t && systemctl reload nginx
certbot --nginx --non-interactive --redirect \
  -d api.$ENV.75-119-147-69.sslip.io -d admindashboard.$ENV.75-119-147-69.sslip.io
```

certbot edits the installed site file to add the 443 blocks and the
HTTP→HTTPS redirect. When you change routing later, edit the installed file
in place (and mirror the change in the repo); don't copy the repo file over
it. Certificates renew automatically.

The long sslip.io hostnames need a larger nginx server-name table. This is
set once per server in its own file, so the main `nginx.conf`, which the
other sites also use, stays untouched:

```bash
printf 'server_names_hash_bucket_size 128;\n' > /etc/nginx/conf.d/server-names-hash.conf
```

Without it, `nginx -t` fails with "could not build server_names_hash".

Production's certificate covers all four of its names:

```bash
certbot --nginx --non-interactive --redirect --expand --cert-name api.production.75-119-147-69.sslip.io \
  -d api.production.75-119-147-69.sslip.io -d admindashboard.production.75-119-147-69.sslip.io \
  -d api.rvpay.co -d admindashboard.rvpay.co
```

### 6.6 First admin user

Admins are database-managed users. There's no signup and no env-var admin.

```bash
/root/rvpay-create-admin.sh testing      # or production
```

It prompts for name, email and password (input is hidden), hashes the
password with the repo's `clients/cmd/hash-password` tool (Argon2id), and
inserts a `USER_ROLE_ADMIN` row into that environment's `clients.users`. The
script lives on the server only. It follows the procedure documented in
`clients/cmd/hash-password/main.go`.

---

## 7. Using the dashboard

1. Open the environment's dashboard `/settings` page (no login needed).
2. Select the matching environment:
   - on `admindashboard.rvpay.co`: **Production** (`api.rvpay.co`);
   - on the testing sslip.io dashboard: **Contabo Testing**;
   - on the production sslip.io dashboard: **Contabo Production**.

   The default is **Testing** (AWS, `api.rvpay.xyz`), which rejects these
   origins, and sign-in fails with "clients could not be reached from this
   browser".
3. Go to `/sign-in` and log in.

The selection is stored per browser and per site, so you pick it once on each
dashboard.

---

## 8. Day-to-day operations

```bash
$C ps -a                              # status (migration jobs show "Exited (0)", that's normal)
$C logs -f --tail=100 clients         # clients | transactions | admindashboard | postgres
$C restart transactions
systemctl status rvpay@$ENV --no-pager
```

Health checks (use the environment's ports):

```bash
curl -s http://127.0.0.1:8080/v1/public/clients/healthcheck        # testing; production 8090
curl -s http://127.0.0.1:8081/v1/public/transactions/healthcheck   # testing; production 8091
```

### Logs

SSH in (`ssh rvpay-vps`), then set `$ENV` and `$C` as in section 3:

```bash
ENV=production        # or testing
cd /opt/rvpay-$ENV
C="docker compose -p rvpay-$ENV -f docker-compose.contabo.yml -f docker-compose.contabo.hostnginx.yml"
```

**App logs** (clients, transactions, dashboard, postgres):

```bash
$C logs -f --tail=100 clients                 # follow live; Ctrl+C to stop
$C logs -f --tail=100 clients transactions    # both services together
$C logs --since 1h transactions               # last hour
$C logs --since 2026-10-08T09:00:00 clients   # since a specific time (UTC)
$C logs admindashboard                        # Next.js dashboard
$C logs postgres                              # database
$C logs migration-clients                     # result of the last migration run
```

**Finding problems.** The Go services log one JSON line per event
(zerolog), so filter by level:

```bash
$C logs --since 24h --no-log-prefix clients transactions | grep -E '"level":"(error|fatal)"'
$C logs --since 24h --no-log-prefix clients transactions | grep '"level":"warn"'
```

`jq` is installed on the server, which makes the lines readable:

```bash
$C logs --since 1h --no-log-prefix transactions | jq -rR 'fromjson? | "\(.time) \(.level) \(.message) \(.error // "")"'
```

To search for one thing, such as a deposit ID or a client ID:

```bash
$C logs --since 24h --no-log-prefix clients transactions | grep '<deposit-or-client-id>'
```

**Web traffic (nginx).** These logs are shared with the other two sites on
the box, so filter for RVPay's hostnames:

```bash
tail -f /var/log/nginx/access.log | grep -E 'rvpay\.co|sslip\.io'    # live requests
tail -50 /var/log/nginx/error.log                                    # proxy errors (e.g. 502 if a service is down)
```

**Server and service-level logs:**

```bash
systemctl status rvpay@$ENV --no-pager     # did the stack start at boot?
journalctl -u rvpay@$ENV --since today     # its start/stop history
journalctl -u docker --since today         # Docker itself
```

**Without SSHing in first,** from an operator PC:

```powershell
ssh rvpay-vps "docker logs --tail 100 rvpay-production-clients-1"
ssh rvpay-vps "docker logs -f rvpay-production-transactions-1"
```

Container names follow the pattern `rvpay-<env>-<service>-1`, e.g.
`rvpay-testing-admindashboard-1`.

**Good to know:**

- **How far back logs go:** each container keeps about 30 MB of logs (3 ×
  10 MB, set in `/etc/docker/daemon.json`). Older lines are deleted
  automatically, so the disk can't fill. For long-term history or alerting,
  add a log service later.
- **Recreating a container clears its logs.** That happens on every deploy,
  so if you're investigating a problem, save the logs before deploying:
  `$C logs --no-log-prefix clients > /root/clients-$(date +%F-%H%M).log`
- **Times are in UTC.**

### Database access

```bash
$C exec postgres sh -c 'psql -U "$POSTGRES_USER" -d clients'        # or -d transactions
```

Postgres has no host port. It's only reachable inside the Compose network
or via `exec`.

### Backups

There are no automated backups yet. Manual dump:

```bash
mkdir -p /root/backups
for db in clients transactions; do
  $C exec -T postgres sh -c "pg_dump -U \"\$POSTGRES_USER\" -Fc $db" > /root/backups/$ENV-$db-$(date +%F).dump
done
```

Data lives in the Docker volume `rvpay-<env>_postgres-data`. **Never run
`$C down -v`.** The `-v` deletes that volume.

### After a reboot

`rvpay@testing` and `rvpay@production` bring both stacks back up, and PM2
brings back the other two sites:

```bash
systemctl status 'rvpay@*' --no-pager; docker ps; pm2 list
```

---

## 9. Problems we hit (and the fixes)

| Symptom | Cause | Fix |
|---|---|---|
| `up` would fail or break the other sites | Bundled nginx container wanted ports 80/443, already used by host nginx | `docker-compose.contabo.hostnginx.yml` disables it and binds gateways to `127.0.0.1` |
| clients exits at startup | `.env.example` had `HIGHLEVEL_REDIRECT_URI`; code reads `HIGHLEVEL_REDIRECT_URL` (required) | Renamed in `.env.example` |
| Webhook key unparseable | Multi-line PEM unquoted in `.env` | Value double-quoted |
| Migrations: `database "clients" does not exist` | Init script used `psql -c` with `\gexec`, which psql doesn't allow | Script now feeds SQL via stdin. Init scripts only run on an empty volume, so on an existing volume run it once by hand: `$C exec -T postgres sh /docker-entrypoint-initdb.d/001-create-service-databases.sh` |
| Sign-in 404 through nginx | `/v1/public/auth/*` missing from the clients route (also missing in the AWS ALB rules) | Added `auth` to the prefix list in every nginx config |
| "clients could not be reached from this browser" | Dashboard still on the AWS Testing environment | Select the Contabo environment in Settings (section 7) |
| "Welcome to nginx!" page | Hit before the site and certificate were installed, or a cached response | Hard refresh (Ctrl+F5) |
| systemd unit missing after clone | `.gitignore` ignored every `systemd/` dir | Added `!deploy/contabo/systemd/` |
| CI image push rejected | GHCR needs lowercase names; the owner is `MountainHubTech` | Workflow lowercases the owner |
| `nginx -t`: "could not build server_names_hash" | sslip.io hostnames longer than the default 64-byte bucket | `/etc/nginx/conf.d/server-names-hash.conf` (section 6.5) |
| New domain works from the server but not from your PC | Your PC or router cached "not found" from before the DNS records existed | `ipconfig /flushdns`, or restart the router/hotspot, or wait a few minutes |
| Dashboard on `admindashboard.rvpay.co` can't sign in | "Production" isn't selected in Settings | Select **Production**. It points at `api.rvpay.co`. |

---

## 10. Cutting over to the real domains

**Production is on `rvpay.co` (2026-10-07).** The domain wasn't configured
in AWS, so nothing moved. DNS is at Namecheap (**Domain List → Manage →
Advanced DNS → Host Records**):

| Type | Host | Value | TTL |
|---|---|---|---|
| A Record | `api` | `75.119.147.69` | Automatic |
| A Record | `admindashboard` | `75.119.147.69` | Automatic |

`@` (`rvpay.co`) and `www` were deliberately left on Namecheap parking,
because nothing in RVPay uses them. Don't point them at the server without
also adding an nginx site for them, or visitors get "Welcome to nginx!". To
make `rvpay.co` redirect to the dashboard, add a redirect site and a
certificate for those names first. Production's `.env` URLs (`PUBLIC_BASE_URL`,
`HIGHLEVEL_REDIRECT_URL`, `HIGHLEVEL_QUERY_URL`, `HIGHLEVEL_PAYMENT_URL`)
point at `rvpay.co`. The HighLevel app must be updated to match (step 6).

**Testing is on `rvpay.xyz` (2026-10-09).** The domain is registered at
Namecheap. Its DNS was hosted in AWS Route 53 (testing account), with the
`api` and `admindashboard` records as aliases to the AWS testing load
balancer. Both were changed to `A 75.119.147.69` (TTL 60) in Route 53, and
the nameservers were then moved to **Namecheap BasicDNS**, with the same
two A records under Advanced DNS. Both providers give the same answer, so
the nameserver move (which can take up to 48 hours to spread) causes no
interruption. After that, the Route 53 hosted zone can be deleted. The zone
had no MX or TXT records. Moving off Route 53 means AWS testing's
CloudFormation root record and its ACM certificate validation records no
longer work, which is fine as AWS testing is being retired.

Testing's `.env` URLs already pointed at `rvpay.xyz`, and its
`HTTP_CORS_ALLOWED_ORIGINS` includes `https://admindashboard.rvpay.xyz`.
Testing's certificate covers all four of its names.

Generic steps, per environment:

1. Decide the mapping: testing → `api.rvpay.xyz` / `admindashboard.rvpay.xyz`,
   production → `api.rvpay.co` / `admindashboard.rvpay.co` (same as
   `cd/environments.yaml`).
2. Migrate any data needed from AWS (pg_dump/restore into the matching
   environment).
3. Add the real names to each nginx site's `server_name` lines, run
   `nginx -t && systemctl reload nginx`, then lower the DNS TTL and point the
   A records at `75.119.147.69`.
4. `certbot --nginx -d api.<domain> -d admindashboard.<domain>` per environment.
5. In each `.env`, update `PUBLIC_BASE_URL`, `HIGHLEVEL_REDIRECT_URL`,
   `HIGHLEVEL_QUERY_URL`, `HIGHLEVEL_PAYMENT_URL` and
   `HTTP_CORS_ALLOWED_ORIGINS`, then `$C up -d`.
6. Update the HighLevel Marketplace app(s) and PawaPay callback URLs.
7. The dashboard's existing `testing`/`production` entries already point at
   `api.rvpay.xyz` / `api.rvpay.co`, so the temporary `contabo-*` entries
   can then be removed.
8. Retire the AWS stacks once traffic has moved.

---

## 11. Leftovers from the original single-stack setup

Before the testing/production split (2026-10-07), one stack ran from
`/opt/rvpay-go` as Compose project `rvpay`. Its data was copied into
testing. These are kept until we're sure nothing is missing, and can then
be removed:

| Leftover | State |
|---|---|
| `/opt/rvpay-go` (checkout and `.env`) | Unused |
| Docker volume `rvpay_postgres-data` | Unused. Data is in testing now. |
| `rvpay-go.service` | Disabled |
| `/root/backups/pre-split-{clients,transactions}-2026-10-07.dump` | The dump used to seed testing |
| `/root/backups/production.env.*`, `/root/nginx-backup/` | Copies taken before edits |

The old hostnames (`api.` / `admindashboard.75-119-147-69.sslip.io`) and
their certificate were removed.

---

## 12. Open items

- [ ] **Production has no admin user yet:**
      `/root/rvpay-create-admin.sh production`, with a strong password.
- [ ] **HighLevel app:** production's `.env` now uses
      `https://api.rvpay.co/oauth/callback`,
      `https://api.rvpay.co/payments/custom-provider/query` and
      `https://admindashboard.rvpay.co/payment`. Update the HighLevel
      Marketplace app to match, or OAuth installs and payment queries won't
      reach production.

- [ ] **PawaPay keys:** testing should use sandbox keys
      (`https://api.sandbox.pawapay.io`); production uses live keys.
- [ ] **HighLevel:** both environments currently share one HighLevel app.
      Ideally, use a separate test app for testing.
- [ ] Disable SSH password login (`PasswordAuthentication no`) and install
      fail2ban. Key login is already confirmed working.
- [ ] Reboot for pending kernel/package updates (quiet time; briefly takes
      every site down).
- [ ] Automated Postgres backups (production first).
- [ ] Optionally: a non-root `deploy` user for SSH and CI.
- [ ] Production on its own VPS once it has real traffic.

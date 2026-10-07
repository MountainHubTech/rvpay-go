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
or the AWS/Render workflows. The real domains (`rvpay.xyz`, `rvpay.co`) still
point at AWS until we cut over (section 10).

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
| Dashboard | https://admindashboard.testing.75-119-147-69.sslip.io | **https://admindashboard.rvpay.co** (also `admindashboard.production.75-119-147-69.sslip.io`) |
| API | https://api.testing.75-119-147-69.sslip.io | **https://api.rvpay.co** (also `api.production.75-119-147-69.sslip.io`) |
| Dashboard environment to select | **Contabo Testing** | **Production** on `admindashboard.rvpay.co`; **Contabo Production** on the sslip.io host |
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

Each Compose project has its own network, containers and Postgres volume.
Testing cannot reach production's database, and the reverse.

API routing (same prefixes in every `nginx*.conf`):

| Path prefix | Goes to |
|---|---|
| `/v1/public/auth`, `/v1/public/clients`, `/v1/public/integrations`, `/v1/public/platforms`, `/oauth/callback`, `/payments/custom-provider`, `/webhooks/highlevel` | clients |
| `/v1/public/merchants`, `/customers`, `/deposits`, `/payments`, `/payouts`, `/transactions` | transactions |
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

Deploys (`git checkout`) never touch `.env`. After editing it, apply with
`$C up -d`, which recreates only the containers whose config changed.

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
nice -n 10 $C up -d --build
```

This builds locally as `rvpay/<service>:local` images, instead of pulling
from GHCR.

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
2. Select **Contabo Testing** or **Contabo Production**. The default is
   **Testing** (AWS, `api.rvpay.xyz`), which rejects these origins, and
   sign-in fails with "clients could not be reached from this browser".
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

---

## 10. Cutting over to the real domains

**Production is on `rvpay.co` (2026-10-07).** The domain wasn't configured
in AWS, so nothing moved. DNS is at Namecheap (Advanced DNS): A records
`api` and `admindashboard` → `75.119.147.69`. The `@` and `www` records are
still Namecheap parking. Production's `.env` URLs (`PUBLIC_BASE_URL`,
`HIGHLEVEL_REDIRECT_URL`, `HIGHLEVEL_QUERY_URL`, `HIGHLEVEL_PAYMENT_URL`)
point at `rvpay.co`. The HighLevel app must be updated to match (step 6).

Testing (`rvpay.xyz`) still points at AWS. Move it only when Contabo testing
is properly tested; until then the AWS setup stays live and unchanged.
Steps, per environment:

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

## 11. Open items

- [ ] **PawaPay keys:** testing should use sandbox keys
      (`https://api.sandbox.pawapay.io`); production uses live keys.
- [ ] **HighLevel:** both environments currently share one HighLevel app
      and its URLs still point at `api.rvpay.xyz`. Ideally, use a separate
      test app for testing.
- [ ] Disable SSH password login (`PasswordAuthentication no`) and install
      fail2ban. Key login is already confirmed working.
- [ ] Reboot for pending kernel/package updates (quiet time; briefly takes
      every site down).
- [ ] Automated Postgres backups (production first).
- [ ] Optionally: a non-root `deploy` user for SSH and CI.
- [ ] Production on its own VPS once it has real traffic.

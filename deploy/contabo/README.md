# Contabo VPS deployment

> For how our actual Contabo server is set up and operated (shared host,
> ports, secrets, runbook, open items), see [SETUP.md](SETUP.md).

These artifacts deploy the current Clients and Transactions gRPC/HTTP
services, the admindashboard Next.js app, PostgreSQL, and a TLS-terminating
nginx proxy on a single Contabo VPS. It mirrors the OCI Compose stack
(`docker-compose.yml`) but targets a plain x86_64 host and the current
services instead of the legacy `deposits` service.

Routing mirrors the real AWS ALB model
(`infra/cloudformation/services/{clients,transactions,admindashboard}.yaml`),
**not** one subdomain per backend service: `clients` and `transactions`
share a single API hostname, split by path prefix; `admindashboard` gets its
own hostname. See `nginx.conf` for the exact prefixes.

## Before deployment

1. Provision a Contabo VPS (any plan with enough headroom for Postgres +
   three app containers; the resource `limits` below total 3 vCPU / ~2.75 GB
   and can be tuned to the plan you pick). Note its public IPv4.
2. Point DNS A records for `api.your-domain.com` and
   `admindashboard.your-domain.com` at that IP.
3. On the VPS: install Docker Engine + the Compose plugin, then restrict the
   firewall (e.g. `ufw`) to allow inbound TCP 22 (ideally from an admin IP
   only), 80, and 443.
4. Clone this repository to `/opt/rvpay-go` (or copy it over), checked out at
   the branch/tag you intend to run.
5. Copy `deploy/contabo/.env.example` to `/opt/rvpay-go/.env` and fill in
   every placeholder. Keep it `chmod 600`, owned by the deployment user.
6. Obtain a TLS certificate covering both hostnames (a SAN cert via
   `certbot --nginx -d api.your-domain.com -d admindashboard.your-domain.com`,
   or a wildcard cert) and place the chain/key at
   `/opt/rvpay-go/certs/fullchain.pem` and `/opt/rvpay-go/certs/privkey.pem`.
7. Edit `deploy/contabo/nginx.conf`, replacing `api.your-domain.com` and
   `admindashboard.your-domain.com` with your real hostnames.
8. **Update `admindashboard/lib/environments.ts`** — the dashboard's
   `clientsBaseUrl`/`transactionsBaseUrl` for `testing`/`production` are
   hardcoded TypeScript constants, not env vars (see "Known gaps" below).
   Either add a new environment entry pointing at `api.your-domain.com`, or
   repoint an existing one, before the dashboard will reach the
   Contabo-hosted API. This is a code change requiring a rebuild, not a
   Compose/env change.
9. Install the systemd unit so the stack survives reboots:
   `sudo cp deploy/contabo/systemd/rvpay-go.service /etc/systemd/system/`,
   then `sudo systemctl daemon-reload && sudo systemctl enable --now rvpay-go`.

## Hosts that already run nginx

If the VPS already serves other sites from a host-level nginx on 80/443
(the case on the current Contabo box), the bundled `nginx` container cannot
bind those ports. Layer `docker-compose.contabo.hostnginx.yml` on top of the
base file: it disables the bundled proxy and publishes the gateways on
loopback only, at the host ports set in `.env` (`CLIENTS_HOST_PORT`,
`TRANSACTIONS_HOST_PORT`, `ADMINDASHBOARD_HOST_PORT`; defaults 8080 / 8081 /
3002), for the host nginx to proxy to with the same path split as
`nginx.conf`:

    docker compose -p rvpay-testing -f docker-compose.contabo.yml -f docker-compose.contabo.hostnginx.yml up -d

Several environments can run side by side this way, each with its own
checkout, `.env`, project name (`-p`) and host ports. The `rvpay@.service`
unit and `deploy-contabo.yml` both work like this. In that mode, steps 6–7
above apply to `deploy/contabo/nginx-host-site.<env>.conf` instead of
`deploy/contabo/nginx.conf`. See [SETUP.md](SETUP.md) for our server.

## First boot

`docker compose -f docker-compose.contabo.yml up -d` (or the systemd unit)
brings up, in order: `postgres` (and its one-time init script, which creates
the `clients` and `transactions` databases on a shared instance), the two
one-shot `migration-clients`/`migration-transactions` jobs, then the
`clients`, `transactions`, and `admindashboard` app containers, then `nginx`.

Both Go app containers run with `RUN_MIGRATIONS=false`; only the dedicated
migration jobs apply schema changes, which avoids migration races if either
service is ever scaled to multiple replicas. `admindashboard` has no database
and no migrations — it only calls the two backend services over HTTP.

## Service-to-service gRPC

`clients` and `transactions` call each other directly over plaintext gRPC
inside the Compose network (`transactions:50051` / `clients:50051`) — this
traffic never goes through nginx:

- `clients` dials `transactions` (`TRANSACTIONS_GRPC_ADDR`) to correlate
  HighLevel Custom Payment Provider queries/webhooks with deposits.
- `transactions` dials `clients` (`CLIENTS_GRPC_ADDR`) to validate admin
  access tokens and to run its GHL sync worker.

gRPC clients connect lazily, so there is no circular `depends_on` between the
two services; each only waits on its own migration job.

## CI/CD secrets

`.github/workflows/deploy-contabo.yml` is manual-trigger only
(`workflow_dispatch`) until Contabo is the agreed cutover target — it does
not yet run automatically on push, so it won't race the live Render pipeline.
It currently builds and deploys `clients` and `transactions` only; it does
not yet build/push `admindashboard`. It requires three repository secrets:
`CONTABO_HOST`, `CONTABO_USER`, and `CONTABO_SSH_PRIVATE_KEY`.

## Known gaps / things to double check before relying on this

- **`admindashboard/lib/environments.ts` hardcodes backend URLs** per
  environment (`local`/`testing`/`production`) as TypeScript constants, with
  the environment selected client-side via `localStorage`. There is no env
  var that controls this at runtime or build time — the AWS task definition
  sets `NEXT_PUBLIC_BASE_URL`, but nothing in the dashboard's source actually
  reads that variable, so it has no effect. Pointing the dashboard at the
  Contabo-hosted API requires editing `environments.ts` and rebuilding the
  image; it is not a Compose/env change.
- `clients/.env.example` and `transactions/.env.example` both document an
  HTTP gateway port as `PORT`, but the code actually reads `HTTP_PORT`
  (`os.Getenv("HTTP_PORT")` in both `cmd/grpc-service/main.go`). This Compose
  file sets `HTTP_PORT` explicitly, so it isn't affected, but the `.env.example`
  files are misleading as local-dev documentation and are worth fixing
  separately.
- The AWS ALB rule for `clients` (`infra/cloudformation/services/clients.yaml`)
  has no path-pattern for `/webhooks/highlevel*` — only `/oauth/callback*` and
  `/payments/custom-provider*` are routed there. The Contabo `nginx.conf`
  deliberately includes `/webhooks/highlevel*` so the route actually works;
  this is an intentional deviation from (and likely a fix for) the AWS config,
  not an oversight.
- This stack assumes a single VPS with no load balancer/failover — it's a
  direct analogue of the OCI Always-Free single-instance design, not a
  high-availability setup.
- `HIGHLEVEL_REDIRECT_URI` and `PUBLIC_BASE_URL` (Clients) must point at the
  real `api.your-domain.com` host once DNS/TLS are live, or the HighLevel
  OAuth/Custom Payment Provider flows will fail.

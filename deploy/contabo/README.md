# Contabo VPS deployment

These artifacts deploy the current Clients and Transactions gRPC/HTTP
services, PostgreSQL, and a TLS-terminating nginx proxy on a single Contabo
VPS. It mirrors the OCI Compose stack (`docker-compose.yml`) but targets a
plain x86_64 host and the two active services instead of the legacy
`deposits` service.

## Before deployment

1. Provision a Contabo VPS (any plan with enough headroom for Postgres + two
   Go services; the resource `limits` below total 2.5 vCPU / ~2.25 GB and can
   be tuned to the plan you pick). Note its public IPv4.
2. Point DNS A records for your API hostnames (e.g. `api.your-domain.com`
   and `transactions.your-domain.com`) at that IP.
3. On the VPS: install Docker Engine + the Compose plugin, then restrict the
   firewall (e.g. `ufw`) to allow inbound TCP 22 (ideally from an admin IP
   only), 80, and 443.
4. Clone this repository to `/opt/rvpay-go` (or copy it over), checked out at
   the branch/tag you intend to run.
5. Copy `deploy/contabo/.env.example` to `/opt/rvpay-go/.env` and fill in
   every placeholder. Keep it `chmod 600`, owned by the deployment user.
6. Obtain a TLS certificate covering both hostnames (a SAN cert via
   `certbot --nginx -d api.your-domain.com -d transactions.your-domain.com`,
   or a wildcard cert) and place the chain/key at
   `/opt/rvpay-go/certs/fullchain.pem` and `/opt/rvpay-go/certs/privkey.pem`.
7. Edit `deploy/contabo/nginx.conf`, replacing `api.your-domain.com` and
   `transactions.your-domain.com` with your real hostnames.
8. Install the systemd unit so the stack survives reboots:
   `sudo cp deploy/contabo/systemd/rvpay-go.service /etc/systemd/system/`,
   then `sudo systemctl daemon-reload && sudo systemctl enable --now rvpay-go`.

## First boot

`docker compose -f docker-compose.contabo.yml up -d` (or the systemd unit)
brings up, in order: `postgres` (and its one-time init script, which creates
the `clients` and `transactions` databases on a shared instance), the two
one-shot `migration-clients`/`migration-transactions` jobs, then the
`clients` and `transactions` app containers, then `nginx`.

Both app containers run with `RUN_MIGRATIONS=false`; only the dedicated
migration jobs apply schema changes, which avoids migration races if either
service is ever scaled to multiple replicas.

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
It requires three repository secrets: `CONTABO_HOST`, `CONTABO_USER`, and
`CONTABO_SSH_PRIVATE_KEY`. It builds both services as `linux/amd64` images,
pushes them to GHCR, then SSHes in and runs `docker compose pull`/`up -d`
against the SHA-tagged images.

## Known gaps / things to double check before relying on this

- `clients/.env.example` and `transactions/.env.example` both document an
  HTTP gateway port as `PORT`, but the code actually reads `HTTP_PORT`
  (`os.Getenv("HTTP_PORT")` in both `cmd/grpc-service/main.go`). This Compose
  file sets `HTTP_PORT` explicitly, so it isn't affected, but the `.env.example`
  files are misleading as local-dev documentation and are worth fixing
  separately.
- This stack assumes a single VPS with no load balancer/failover — it's a
  direct analogue of the OCI Always-Free single-instance design, not a
  high-availability setup.
- `HIGHLEVEL_REDIRECT_URI` and `PUBLIC_BASE_URL` (Clients) must point at the
  real `api.your-domain.com` host once DNS/TLS are live, or the HighLevel
  OAuth/Custom Payment Provider flows will fail.

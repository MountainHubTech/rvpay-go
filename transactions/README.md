# Transactions service

The Transactions service is the `rvpay-go` microservice that owns the payment
transaction domain: merchants, customers, deposits, and payouts. It exposes
gRPC operations plus a REST/gRPC-gateway surface, persists transaction records
to PostgreSQL, and is wired for future provider integration.

## Purpose

The Transactions service implements the documented RVPay transaction
capabilities:

- **Merchants** — payment gateway records (create, get, list).
- **Customers** — end-user payment records tied to a client and merchant
  (create, get).
- **Deposits** — inbound customer payments (initiate, get).
- **Payouts** — outbound settlements (request, get).

It follows the domain model in `docs/domain-model.md` and the layout in
`docs/repository-layout.md`.

## Responsibilities

- Own the Transactions PostgreSQL database (merchants, customers, deposits,
  payouts).
- Expose the Transactions gRPC API (`transactionsgrpc`) and REST gateway.
- Validate merchant/customer ownership and financial record integrity.
- Apply database migrations at startup when enabled.
- Remain provider-agnostic: no external payment provider is called from this
  service yet.

## Service Structure

```text
transactions/
├── cmd/grpc-service/main.go      # gRPC + gateway server bootstrap and shutdown
├── config/model.go               # Config and DBConfig environment bindings
├── db/
│   ├── migrations/               # 000001 creates merchants, customers, deposits, payouts
│   ├── query/                    # SQL inputs consumed by sqlc
│   ├── repo/                     # pgx pool adapter plus domain repositories
│   ├── sqlc/                     # sqlc-generated data access code
│   └── doc.go                    # go:generate directives (sqlc + mocks)
├── merchants/                    # MerchantService implementation
├── customers/                    # CustomerService implementation
├── deposits/                     # DepositService implementation
├── payouts/                      # PayoutService implementation
├── Dockerfile                    # Multi-stage distroless build
├── Makefile                      # Local development tasks
├── README.md                     # This file
└── .env.example                  # Runtime environment template
```

## Running Locally

Run all commands below from the `transactions/` directory.

1. Configure the environment:

```bash
cp .env.example .env
# edit .env with your local PostgreSQL values
```

2. Start PostgreSQL:

```bash
make rundb
docker exec -it transactions-postgres psql -U postgres -d transactions -c 'CREATE EXTENSION IF NOT EXISTS pgcrypto;'
```

`make rundb` starts a detached PostgreSQL 16 Alpine container named
`transactions-postgres`, exposes `DB_PORT`, and uses `DB_USER`, `DB_PASSWORD`,
and `DB_NAME` from `.env`.

3. Run the service:

```bash
make run
```

The service loads `.env`, connects and pings PostgreSQL, applies migrations
(when `RUN_MIGRATIONS=true`), registers the four Transactions services with
gRPC reflection and a unary panic-recovery interceptor, then listens on
`:$LISTEN_PORT`. The REST gateway listens on `:$PORT` (default `8080`). It
stops gracefully on `SIGINT` or `SIGTERM`.

The initial schema relies on `gen_random_uuid()`, so `pgcrypto` must be enabled
in the database.

## Configuration

| Variable | Required | Purpose |
| --- | --- | --- |
| `LOG_LEVEL` | No; defaults to `debug` | Zerolog level |
| `LISTEN_PORT` | Yes | gRPC TCP port |
| `MIGRATION_PATH` | Yes | Migration directory, typically `transactions/db/migrations` |
| `RUN_MIGRATIONS` | No; defaults to `true` | Apply migrations at startup when true |
| `DB_USER` | Yes | PostgreSQL user |
| `DB_PASSWORD` | Yes | PostgreSQL password |
| `DB_HOST` | Yes | PostgreSQL host |
| `DB_PORT` | Yes | PostgreSQL port |
| `DB_NAME` | Yes | PostgreSQL database |
| `DB_TLS_DISABLED` | No; defaults to `false` | Selects `sslmode=disable` when true; set `false` for `require` |

No secrets are committed. All credentials are provided through the environment
(`.env` locally, platform environment variables in production).

## Database

- PostgreSQL is required.
- Migrations are owned by `transactions/db/migrations` and applied at startup
  when `RUN_MIGRATIONS=true`.
- SQLC generates the data-access code from `transactions/db/query/*.sql`.
- `db/repo` exposes `TransactionsRepo.Do()`/`Begin()` plus the domain
  repositories (Merchant, Customer, Deposit, Payout).

## Code Generation

Generation ownership:

| Generated Artifact | Command | Owner |
| --- | --- | --- |
| protobuf + gRPC + gateway | `cd protobuf && make generate-protos` | protobuf Makefile |
| SQLC models/queries | `cd transactions/db && go generate ./...` (or `make generate`) | Transactions DB |
| repository/queries mocks | `cd transactions/db && go generate ./...` (mockgen v0.6.0) | Transactions DB |

Run `make generate` from the `transactions/` directory to regenerate SQLC and
mocks. Do not manually edit generated files.

## Testing

```bash
make test
# or
go test ./...
```

Focused Transactions tests:

```bash
go test ./merchants/...
go test ./customers/...
go test ./deposits/...
go test ./payouts/...
go test ./config/...
```

## Docker

```bash
# from the repository root (the Docker build context)
docker build -f transactions/Dockerfile -t rvpay-go-transactions:local .
```

Run with the required environment variables:

```bash
docker run --rm \
  -e LISTEN_PORT=50051 \
  -e DB_USER=postgres -e DB_PASSWORD=secret -e DB_HOST=host.docker.internal \
  -e DB_PORT=5432 -e DB_NAME=transactions -e DB_TLS_DISABLED=true \
  -e MIGRATION_PATH=/app/transactions/db/migrations \
  -e PORT=8080 \
  -p 50051:50051 -p 8080:8080 \
  rvpay-go-transactions:local
```

The container runs as a non-root distroless user and binds to the configured
ports. No secrets are baked into the image.

## gRPC API

The generated gRPC service package is `transactionsgrpc`, registering four
services:

| Service | RPCs |
| --- | --- |
| `MerchantService` | `CreateMerchant`, `GetMerchant`, `ListMerchants` |
| `CustomerService` | `CreateCustomer`, `GetCustomer` |
| `DepositService` | `InitiateDeposit`, `GetDeposit` |
| `PayoutService` | `RequestPayout`, `GetPayout` |

Example with local reflection enabled:

```bash
grpcurl -plaintext \
  -d '{"name":"PawaPay","slug":"pawapay"}' \
  localhost:50051 transactionsgrpc.MerchantService/CreateMerchant
```

The full protobuf schema is `../protobuf/transactions.proto`.

## REST API

The grpc-gateway exposes REST routes generated from the protobuf annotations:

| Method | Route | RPC |
| --- | --- | --- |
| POST | `/v1/public/merchants` | CreateMerchant |
| GET | `/v1/public/merchants/{merchant_id}` | GetMerchant |
| GET | `/v1/public/merchants` | ListMerchants |
| POST | `/v1/public/customers` | CreateCustomer |
| GET | `/v1/public/customers/{customer_id}` | GetCustomer |
| POST | `/v1/public/deposits` | InitiateDeposit |
| GET | `/v1/public/deposits/{deposit_id}` | GetDeposit |
| POST | `/v1/public/payouts` | RequestPayout |
| GET | `/v1/public/payouts/{payout_id}` | GetPayout |

The gateway listens on `:$PORT` (default `8080`) and serves `/healthz`.

## Migrations

- Up/down migrations live in `transactions/db/migrations` (`000001_init_schema`).
- Migrations run automatically at startup when `RUN_MIGRATIONS=true`.
- Create a new migration with:

```bash
make create-migration name=descriptive_migration_name
```

`make create-migration` requires the `migrate` CLI. Destructive rollback is not
wired into the default Makefile targets.

## Architecture Notes

```text
gRPC API (transactionsgrpc)
    ↓
MerchantService / CustomerService / DepositService / PayoutService
    ↓
MerchantRepo / CustomerRepo / DepositRepo / PayoutRepo
    ↓
SQLC (transactions/db/sqlc)
    ↓
PostgreSQL
```

- Each service embeds the generated `Unimplemented*ServiceServer` for forward
  compatibility.
- The Deposit service validates the customer belongs to the declared
  client+merchant+phone context before persisting.
- Payouts are not customer-scoped; they reference client and merchant only.
- Provider execution (PawaPay etc.) is intentionally not wired in this
  service; the deposit/payout lifecycle is ready for a future integration
  boundary.

## Troubleshooting

- **Database connection failure** — verify `DB_USER`, `DB_PASSWORD`, `DB_HOST`,
  `DB_PORT`, `DB_NAME` in `.env`; ensure PostgreSQL is running and reachable.
- **Missing environment variables** — the service requires `LISTEN_PORT`,
  `MIGRATION_PATH`, and the `DB_*` variables; otherwise startup fails with a
  configuration error.
- **Migrations do not apply** — ensure `MIGRATION_PATH` points to
  `transactions/db/migrations` and that `pgcrypto` is enabled in the database.
- **Port conflicts** — the gRPC port (`LISTEN_PORT`) and gateway port (`PORT`)
  must be free; if a port is already in use, startup fails with
  `net.Listen: bind: address already in use`.
- **Generated code is stale** — after changing migrations or SQL, run
  `make generate`; never hand-edit `db/sqlc` output.
- **Docker build** — the build context is the repository root:
  `docker build -f transactions/Dockerfile ..` from `transactions/`.

## PawaPay Integration

The Transactions service initiates transactions with the PawaPay V2 API through
the `github.com/I-Frostbyte/pawapay_client` SDK.

### Environment

Only two PawaPay variables are used. Add them to `.env` (see `.env.example`):

| Variable | Required | Purpose |
| --- | --- | --- |
| `PAWAPAY_API_URL` | No | PawaPay V2 base URL (e.g. `https://api.sandbox.pawapay.io`) |
| `PAWAPAY_API_KEY` | No | PawaPay API key sent as `Authorization: Bearer <key>` |

The client is constructed in `cmd/grpc-service/main.go` via
`pawapay_client.NewClient(config.APIURL, config.APIKey)` and injected into the
deposit and payout services.

### Integrated methods

Only the current PawaPay initiation operations are wired in:

- **Deposits** — `Deposits.InitiateDeposit` is called after the deposit is
  persisted (`deposits/service.go`). The SDK `Payer.Type` is fixed to `MMO`
  and the provider is mapped as `MTN_MOMO` → `MTN_MOMO_CMR`,
  `ORANGE_MOMO` → `ORANGE_MOMO_CMR`. The amount is sent as a decimal string.
- **Payouts** — `Payouts.InitiatePayout` is called after the payout is
  persisted (`payouts/service.go`). The SDK `Recipient.Type` is fixed to `MMO`
  with the same provider mapping.

Provider failures are surfaced as gRPC `INTERNAL` errors. No callbacks,
reconciliation, status polling, retries, or webhooks are implemented.

### Assumption

The payout domain/proto has no dedicated phone-number field; the PawaPay SDK
requires a recipient phone number. The payout `destination_reference` is mapped
to `Recipient.AccountDetails.PhoneNumber`. Confirm that callers always populate
`destination_reference` with a valid mobile-money phone number.
## Recent changes (unpushed local commits)

Changes made to the Transactions service in the most recent three (unpushed)
commits. These are configuration/style-only; no PawaPay domain, SDK, deposit,
payout, or gRPC behavior was changed.

### Configuration (`config/model.go`)
- `Config` and `DBConfig` were restyled to match `clients/config/model.go`
  (field grouping, `DBConfig` comment, `LoadConfig` comment and formatting).
- The `LOG_LEVEL` default changed from `debug` to `info`.
- `RUN_MIGRATIONS` remains `true`. Environment variables are unchanged
  (`LOG_LEVEL`, `LISTEN_PORT`, `MIGRATION_PATH`, `RUN_MIGRATIONS`,
  `PAWAPAY_API_URL`, `PAWAPAY_API_KEY`, and the `DB_*` set).

### Tests
- `config/model_test.go` updated so the defaults test expects the new `info`
  log level (`TestLoadConfigDefaultsApplied`).
### HighLevel Inbound Webhook Delivery (`ghldeliver`)

Status: IMPLEMENTED (2026-09-20). Durable RVPay → HighLevel Inbound Webhook outbound delivery via an outbox-backed `rvpay.payment.completed` event emitted inside the PawaPay COMPLETED callback transaction, delivered asynchronously by a bounded-retry worker to a Secret Manager-injected `HIGHLEVEL_INBOUND_WEBHOOK_URL`.

- Flow: RVPay POST JSON → HighLevel Inbound Webhook → Create/Update Contact → If/Else (rvpay.payment.completed) → If/Else (status==paid) → If/Else (payment already processed?) → Update Contact → Add "RVPay Payment Completed" → Send confirmation → Send SMS. The GHL workflow is manually configured; RVPay implements only its side (emit event for confirmed payment).
- Secret (`HIGHLEVEL_INBOUND_WEBHOOK_URL`): loaded from env with empty default; ignored if missing or non-HTTPS (worker runs disabled without config, payments unaffected, delivery resumes after restart with correct config). In production delivered through AWS Secrets Manager via existing ECS task-definition secret-injection architecture. Never hard-coded, never in DB, never in source-controlled files (only empty placeholder in `.env.example`).
- Event emission: PawaPay COMPLETED callback → `ProcessDepositCallback` → `applyPawaPayCallbackTransition` → `enqueuePaymentCompletedEvent` runs inside the same DB transaction that commits terminal COMPLETED state + GHL sync pending status. Insert is idempotent (unique `deposit_id`); duplicate callback loses race or is no-op.
- Event contract: `transactions/ghldeliver/event.go` `BuildPaymentCompletedEvent`. Exact agreed JSON field names/structure. Authoritative sourcing per field: payment.id, providerTransactionId (threaded from PawaPay callback via `SetExternalReference`→`external_reference`), amount/paidAmount (deposit amount in minor units), currency, status=paid, customer.id/name/email/phone, order.id/status/orderNumber, location.id/name (parsed from `highlevel-<locationId>` client name convention; non-conforming → empty, never fabricated), products[].name/categories/name, now timestamp, idempotencyKey=event id, eventId (UUID), webhookEventId. `customerEmail` and `productName` confirmed not persisted in RVPay → emitted as "" with in-code comment + test asserting they remain empty.
- Outbox/worker: `transactions/ghldeliver/` package. Worker polls `payment_events` (delivery_status='pending'), claims due rows atomically (FOR UPDATE SKIP LOCKED), POSTs immutable payload snapshot to configured URL. Marks delivered only after 2xx. HighLevel availability never determines payment success (async after-the-fact; callback does not wait).
- Retry behavior (bounded): transient (timeout/network/HTTP 5xx/408/429) with attempts<MaxDeliveryAttempts(5) → re-queue 'pending' with backoff 1m/5m/15m/60m; attempts exhausted → 'failed'; permanent HTTP 4xx → 'failed' immediately (no infinite retry for misconfiguration). Retries reuse identical eventId/idempotencyKey/payload bytes from outbox row.
- Idempotency: duplicate provider callbacks don't duplicate logical event (unique deposit_id). Delivery retries reuse same event ID, idempotency key, and payload.
- Secrets/logging: configured webhook URL never logged. HTTP poster strips url.Error messages (contain URL) from transport errors; worker logs config VARIABLE NAME ("HIGHLEVEL_INBOUND_WEBHOOK_URL") on validation failure, never its value.
- Tests: `transactions/ghldeliver/event_test.go`, `transactions/ghldeliver/client_test.go`, `transactions/ghldeliver/worker_test.go`, `transactions/payments/payment_event_test.go`, `transactions/payments/callback_test.go` (updated COMPLETED test expectations to include event enqueue).

#### Files created
- `transactions/db/migrations/000007_payment_events.{up,down}.sql` — payment_events outbox table
- `transactions/db/query/payment_events.sql` — 6 sqlc queries (Insert/Claim/RecordSuccess/Retry/Failure/GetByDepositID)
- `transactions/db/repo/payment_event_repo.go` — PaymentEventRepo interface + implementation
- `transactions/ghldeliver/event.go` — PaymentCompletedEvent, BuildPaymentCompletedEvent, field sourcing, guards
- `transactions/ghldeliver/client.go` — HTTPPoster, Poster interface, DeliveryError classification, ValidateInboundWebhookURL
- `transactions/ghldeliver/worker.go` — Worker, Run/RunOnce, claim/deliver/backoff, disabled-without-config safety
- `transactions/ghldeliver/{event,client,worker}_test.go` — full test suite
- `transactions/payments/payment_event_test.go` — enqueue idempotent/rollback/provider_transaction_id tests
- `infra/cloudformation/components/third_party_secrets.yaml` — HighLevelInboundWebhookSecret resource + output

#### Files modified
- `transactions/db/sqlc/{payment_events.sql.go,models.go,querier.go}` — sqlc generated code (REGENERATED)
- `transactions/db/repo/mocks/repo.go` — mock PaymentEventRepo (REGENERATED)
- `transactions/payments/service.go` — `enqueuePaymentCompletedEvent` method + call site; imports ghldeliver
- `transactions/payments/callback_test.go` — updated COMPLETED callback test expectations to include event enqueue
- `transactions/config/model.go` — `HighLevelInboundWebhookURL` config field (env:HIGHLEVEL_INBOUND_WEBHOOK_URL, optional, empty default)
- `transactions/.env.example` — `HIGHLEVEL_INBOUND_WEBHOOK_URL=` placeholder + comment
- `transactions/cmd/grpc-service/main.go` — ghldeliver worker constructed from config + started in startup goroutine
- `infra/cloudformation/services/transactions.yaml` — `HighLevelInboundWebhookSecretArn` param + secret injection in TaskDefinition Secrets

#### Validation
- `go build ./...` — PASS
- `go vet ./transactions/...` — PASS
- `go test ./transactions/...` — PASS (all new + existing tests)
- `gofmt -l transactions/ transactions/db/` — clean
- No hard-coded URL anywhere; CFN secret + ECS injection in place; worker disabled-without-config safety confirmed by tests

#### Remaining manual steps
- AWS deploy `third_party_secrets.yaml` to create secret `/<EnvId>/highlevel_inbound_webhook`
- Set real HighLevel Inbound Webhook URL value in AWS Secrets Manager (replace placeholder)
- Deploy `transactions.yaml` so ECS task receives secret injection
- Confirm task role has `secretsmanager:GetSecretValue` for the highlevel_inbound_webhook secret ARN
- Live GHL verification of full RVPay → Inbound Webhook → workflow path

## Correct Account & Customer Names — Agent 2026-09-23

### Status: COMPLETE

### Agent/task
`agents/rvpay-correct-account-customer-names-cline-agent.md` — add `display_name` for Clients and `customer_name` for Transactions so the Admin Dashboard shows authoritative HighLevel location/sub-account names and customer contact names instead of raw internal IDs.

### Exact files changed
- `clients/db/query/clients.sql` — new queries `ListClientsWithDisplayNames`, `GetClientDisplayName`
- `clients/db/sqlc/*.go` — regenerated (models + querier)
- `clients/db/repo/client_repo.go` — `DisplayNames()` and `GetDisplayName()` methods
- `clients/service/clients_service.go` — `ListClients` populates `DisplayName` from HighLevel
- `clients/cmd/grpc-service/main.go` — wiring
- `clients/cmd/backfill-client-names/main.go` — standalone backfill CLI (source only; no compiled binary committed)
- `transactions/db/query/customers.sql` — new queries `GetCustomerNamesByID`, `GetCustomerNamesByIDs`
- `transactions/db/sqlc/*.go` — regenerated (models + querier)
- `transactions/db/repo/customer_repo.go` — `GetNamesByID()` and `GetNamesByIDs()` methods
- `transactions/payments/service.go` — customer name lookup in PawaPay callbacks
- `admindashboard/` — frontend wiring to display `display_name` / `customer_name`
- `transactions/db/repo/mocks/repo.go` — regenerated via `go generate` (mockgen)
- `transactions/db/sqlc/mocks/querier.go` — regenerated via `go generate` (mockgen)

### Account-name source
HighLevel Location name resolved from the `locations.readonly` scope via the existing platform lookup (`slug='highlevel'`) and the location cache populated during OAuth/token refresh. Client `client_name` is stored as `highlevel-<locationId>`; the display name is the human-readable HighLevel location name fetched from the HighLevel API.

### Customer-name source
HighLevel Contact name resolved from the HighLevel API using the contact ID stored in the `customers` table. The `customer_name` field on transactions is populated from the HighLevel contact's name at callback processing time.

### `locations.readonly` dependency
The account-name resolution depends on the HighLevel `locations.readonly` scope being granted during OAuth installation. If this scope is missing, display names cannot be resolved and the system falls back to the `highlevel-<locationId>` convention.

### Client backfill mechanism
`clients/cmd/backfill-client-names/main.go` — a standalone CLI tool that iterates all clients, resolves display names from HighLevel, and updates the database. Run with `go run ./clients/cmd/backfill-client-names`. The compiled binary is NOT committed to the repository (only `main.go` is source-controlled).

### Customer backfill mechanism
Customer names are populated lazily during PawaPay callback processing. For existing transactions without customer names, a separate backfill pass can be implemented using the `GetCustomerNamesByIDs` batch query.

### Protobuf/API changes
None that break existing contracts. The `display_name` and `customer_name` fields are populated server-side and returned in existing response types. No new RPCs added to the public API surface; the fields are available through existing List/Create/Get methods.

### Dashboard changes
Admin Dashboard transaction list and client list now display `display_name` (human-readable HighLevel location name) instead of/in addition to the raw `highlevel-<locationId>` client_name. Customer names shown in transaction details.

### Database changes
- `clients` table: `display_name` populated in application layer from HighLevel API (no new migration required unless persistence is desired)
- `transactions` / `deposits`: `customer_name` field populated from HighLevel contact data at callback time

### Manual backfill execution instructions
```bash
# Client display name backfill
go run ./clients/cmd/backfill-client-names

# Customer name backfill (lazy, happens during callbacks; manual batch if needed)
# uses GetCustomerNamesByIDs batch query
```

### OAuth reauthorization implications
If the HighLevel OAuth integration is reauthorized, the location cache must be re-populated. The `locations.readonly` scope must be included in the OAuth scope request. If scopes change, display name resolution may fail until reauthorization completes.

### Tests/results
- `go test ./clients/...` — PASS (all packages)
- `go test ./transactions/...` — PASS (all packages, after mock regeneration)
- `go test ./...` — PASS

### Build/lint/vet results
- `go build ./...` — PASS
- `go vet ./clients/... ./transactions/...` — PASS
- `gofmt` clean on changed files

### Known limitations
- Display names require HighLevel API access and `locations.readonly` scope
- Customer names depend on HighLevel contact data being available
- Backfill is not automatic for historically created records
- If HighLevel API is unavailable, display names may be stale or missing
- The compiled backfill binary is not committed; only source is tracked
- Customer name backfill for existing transactions requires explicit batch run

### Records that could not be corrected automatically
- Clients created before this feature without HighLevel name resolution
- Transactions processed before customer name lookup was added
- Records for HighLevel locations no longer accessible via API

### Next task
- Deploy and verify with live HighLevel API
- Consider adding database columns for `display_name` and `customer_name` if persistence is required
- Monitor backfill completeness

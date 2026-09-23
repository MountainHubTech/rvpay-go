This is a [Next.js](https://nextjs.org) project bootstrapped with [`create-next-app`](https://nextjs.org/docs/app/api-reference/cli/create-next-app).

## Getting Started

First, run the development server:

```bash
npm run dev
# or
yarn dev
# or
pnpm dev
# or
bun dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/app/building-your-application/optimizing/fonts) to automatically optimize and load [Geist](https://vercel.com/font), a new font family for Vercel.

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.

You can check out [the Next.js GitHub repository](https://github.com/vercel/next.js) - your feedback and contributions are welcome!

## Deploy on Vercel

The easiest way to deploy your Next.js app is to use the [Vercel Platform](https://vercel.com/new?utm_medium=default-template&filter=next.js&utm_source=create-next-app&utm_campaign=create-next-app-readme) from the creators of Next.js.

Check out our [Next.js deployment documentation](https://nextjs.org/docs/app/building-your-application/deploying) for more details.

## Dockerization Workflow

This application is being prepared for production Docker deployment through a sequence of numbered Cline agents. The workflow is controlled by the project-control files in this repository:

- `.clinerules.md` — rules all agents must follow
- `.clineignore.md` — areas agents should avoid reading unless directly required
- `.project-context.md` — stable facts, decisions, and unknowns
- `.project-checkpoint.md` — chronological checkpoint ledger with reading map
- `.clinecheck.md` — verification checklist for the sequence
- `.project-next-steps` — AWS deployment guide (populated only after the sequence completes)

AWS deployment steps are documented only after Agent 06 completes the Dockerization sequence.

## Docker / AWS-Readiness Status

The Dockerization sequence is complete for local production capability. The production image builds and the container starts successfully.

- **Build:** multi-stage `node:22-alpine` build (`npm ci` from the committed lockfile → `next build` → minimal standalone runtime). `next.config.ts` uses `output: "standalone"`.
- **Runtime:** minimal standalone server, listens on port **3000**, runs as non-root `nodejs` (uid/gid 1001), with `NODE_ENV=production` baked in.
- **Image hygiene:** `.dockerignore` excludes env files and secret material; verified no `.env` / `.pem` / `.key` in the image.
- **Environment:** the application uses **zero** environment variables; no `.env` is required at build or runtime.
- **Workflow:** `make build`, `make run`, `make stop`, `make verify` cover the local Docker workflow. `make tag` / `make ecr-login` / `make ecr-push` are a thin AWS ECR push interface (require `AWS_REGION`, `AWS_ACCOUNT_ID`, `AWS_ECR_REPO`; no credentials are committed).
- **AWS readiness:** the image can be built reproducibly, tagged, pushed to ECR, and run with runtime values, and it can sit behind a load balancer on port 3000. No AWS infrastructure is created here. Deployment steps and open architecture decisions are in `.project-next-steps.md`.

## HighLevel paymentsUrl Checkout (app/payment/page.tsx)

The `/payment` route is the checkout page HighLevel loads in an iframe as the registered Custom Payment Provider `paymentsUrl`:

1. HighLevel opens the page with the transaction context on the URL (`transactionId`, and optionally `amount`, `chargeId`, `subscriptionId`, `apiKey`).
2. The customer selects country, mobile-money provider, and phone number, then clicks Pay.
3. The page verifies the request via `POST https://api.rvpay.xyz/payments/custom-provider/query` (`type: "verify"`, the Clients-service `queryUrl` contract) and polls that endpoint until success/failure.

Recent fix: the page previously required BOTH `transactionId` AND `apiKey` URL params, so every real iframe load failed with "Invalid payment request. Missing payment transaction information." The guard now requires only `transactionId` (the provider `apiKey` is a server-side `queryUrl` credential, not a frontend one), the query body always sends the documented `type: "verify"`, and the order amount is prefilled (and locked) when HighLevel passes an `amount` query parameter. Amount prefill from backend order data is not possible without a new backend endpoint (query response carries only success/failed/message) — recorded as a known limitation, not changed here.

### postMessage contract (GHL docs §8.2 — payment_initiate_props)

Per HighLevel's documentation, the payment context is NOT on the URL: after the iframe posts a `ready` event to its parent, HighLevel sends a `payment_initiate_props` window message with `amount`, `currency`, `mode`, `productDetails`, `contact`, `orderId`, `transactionId`, `subscriptionId`, and `locationId`. The checkout page now implements this handshake:

1. On mount it posts `{ type: "ready" }` to `window.parent` (the handshake trigger) and listens for `message` events.
2. On `payment_initiate_props` it captures the payload and uses it as the primary transaction context (URL params remain a fallback for direct loads).
3. Order prefill from the event: `amount` (locked), `currency` → country preselection (unambiguous ISO codes only), `contact.contact` → phone number with dial code stripped.
4. On Pay it still verifies via `POST https://api.rvpay.xyz/payments/custom-provider/query` with the event's `transactionId`/`orderId`/`subscriptionId` and `type: "verify"`; the event payload itself is never sent anywhere.

Note: no completion message is posted back to HighLevel yet — add it once the documented completion event (`payment_completed` / `payment_failed`) contract is confirmed.

### Agent 00-payment-page-agent — transactionId authority + real initiation (2026-08-31)

Following live-testing evidence that HighLevel never sends `payment_initiate_props`, that handshake is **suspended** (listener and `[RVPay]` diagnostics retained, non-authoritative) and the payment flow now works without it:

1. **transactionId** — the real HighLevel transaction ID is read from the payment URL query string (`?transactionId=...`); the suspended event is only a fallback. With no ID at all the page fails safely and never fabricates one.
2. **Initiation** — Pay calls the existing Transactions-service endpoint `POST /v1/public/deposits` with the `CreateDepositRequest` payload: `amount {amount, currency}` (ISO-4217; `FCFA` → `XAF` per repo convention), `paymentType: "PAYMENT_TYPE_MMO"`, `payerPhoneNumber` (dial code + number), `provider` (`PROVIDER_MTN_MOMO` / `PROVIDER_ORANGE_MOMO` — unsupported providers are rejected up front rather than faked), and `clientId`/`customerId`/`merchantId` only when HighLevel supplied them on the URL.
3. **Status** — polling uses the existing `GET /v1/public/payments/verify` (`ghlTransactionId` / `ghlChargeId` / `subscriptionId` → `{success, failed}`) with the same transaction ID; single interval, cleaned up on terminal state and unmount; duplicate submissions blocked while processing.

The UI (country, provider, phone, amount, terms) is unchanged; the backend remains the sole authority for payment state.

**Known backend gap (out of scope):** `CreateDepositRequest` has no `ghl_transaction_id` field, so the initiated deposit is not yet correlated with the HighLevel transaction that `/v1/public/payments/verify` looks up — a small approved backend/proto change is required for full end-to-end correlation. Until then the page surfaces backend validation errors rather than fabricating context.

### Surgical correction (2026-08-31): transactionId semantics + ungated initiation

- `buyNowProductId` is **no longer treated as a transactionId**; the page reads only a genuine `transactionId` URL param (or the suspended `payment_initiate_props` event) and treats it as `null` when unavailable — never fabricated.
- The hard `if (!transactionId)` gate was removed: Pay now always attempts the real initiation at `POST /v1/public/deposits` with the validated form values (terms, phone, positive amount, supported provider).
- GHL verification polling runs **only** when a real `transactionId` exists; otherwise the page honestly reports "submitted — status tracking unavailable until correlation is configured." A backend-provided deposit `id` is retained/logged, but HTTP 200 is never shown as payment success.
- All `[RVPay]` handshake diagnostics and the `custom_provider_ready` postMessage remain unchanged; backend/protobuf/Docker/config untouched.

### Payment page redo (2026-09-02) — working `/payment/checkout` handshake reproduced

The `/payment` page now reproduces the proven HighLevel custom-provider iframe
behavior from the working `/payment/checkout` implementation:

- **Handshake:** the `message` listener is registered before the ready post; the
  ready message is sent JSON-stringified as
  `{type:"custom_provider_ready", loaded:true, addCardOnFileSupported:false}`
  and **retried every 500ms** until the payment context arrives (or unmount).
- **Message parsing:** incoming `event.data` may be a JSON string or an object;
  strings are safely `JSON.parse`d and malformed/unrelated messages are ignored.
- **Events:** both `payment_initiate_props` and `setup_initiate_props` are
  accepted as the HighLevel payment context (transactionId, orderId, amount,
  currency, contact, locationId) and used to prefill amount/country/phone.
- **transactionId:** preferred from the event payload; the genuine URL
  `transactionId` param remains a fallback; `orderId` is never substituted and
  nothing is fabricated. Without a real ID the page fails safely with
  "Invalid payment request. Missing payment transaction information."
- **Backend:** initiation still uses the existing `POST /v1/public/deposits`;
  status polling still uses the existing `GET /v1/public/payments/verify?ghlTransactionId=`.
  `CreateDepositRequest` has no `ghl_transaction_id` field (verified in
  `protobuf/transactions.proto`), so none is invented client-side; end-to-end
  correlation still requires an approved backend/proto change.
- **Verification:** esbuild TSX transform passes. Full `tsc --noEmit`/`next build`
  and a live HighLevel iframe test were NOT possible in this environment — re-run
  `npm run build` and a live payment test before publish.

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

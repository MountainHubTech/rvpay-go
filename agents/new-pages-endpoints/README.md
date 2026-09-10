# Missing Backend Endpoints — Transactions / Disputes / Settings Pages

Status snapshot after implementing `admindashboard` pages from
`agents/final-dashboard-pages.md` (2026-09-10). Every item below is real
backend work required before the corresponding UI can show live rows. The UI
already renders truthful empty states and reuses existing endpoints wherever
possible — no mock data and no speculative endpoints were created.

---

## 1. Transactions — transaction list (MISSING)

- Page: Transactions
- UI feature: paginated table (Transaction ID, Sub-Account, Customer, Amount,
  Status, Gateway, Date) with sub-account/status/date-range filters.
- Owning service: Transactions
- Proposed HTTP route: `GET /v1/public/transactions`
  (fits the existing ALB rule `25/50/75` prefix `/v1/public/transactions*` —
  no ALB change required)
- Proposed gRPC: `ListTransactions` on a new/existing TransactionsService in
  `protobuf/transactions.proto`
- HTTP method: GET
- Authentication: Bearer access token — add path to the transport middleware
  protected-path list in `transactions/cmd/grpc-service/main.go` (same list
  that already protects `/v1/public/transactions/overview/snapshot`).
- Request params: `search`, `subAccountId`, `status`, `from`, `to`,
  `page`, `pageSize`
- Response: `{ rows: [{ id, shortId, subAccountId, subAccountName, customerId,
  customerName, amount, currency, status, gateway, createdAt }], total, page,
  pageSize }`
- Sorting: `createdAt desc` default; optional `sort`/`order`.
- Database: requires deposits/payments data joined with merchants/customers;
  existing tables can support it, but deposited-transaction status
  normalization (Success/Failed/Refunded/Pending) must be defined.
- Layers: proto → gateway regen (protobuf/Makefile) → handler → service →
  repository (sqlc query + regen) → middleware protected-path entry.
- CORS: none beyond existing shared middleware (Content-Type,
  Authorization already allowed).
- ALB: covered by existing rule (75 `/v1/public/transactions*`).
- EXISTS today: `GET /v1/public/transactions/overview/snapshot` (stats only —
  used by the page's stat cards), `GET /v1/public/payouts`,
  `GET /v1/public/clients/sub-accounts` (filter options).

## 2. Transactions — transaction statistics refinement (PARTIALLY EXISTS)

- Page: Transactions
- UI feature: "Total Pending" and "Total Cleared (30d)" stat cards.
- Currently satisfied by `GET /v1/public/transactions/overview/snapshot`
  (`pendingPayouts`, `totalRevenue`) — payout-derived, not
  transaction-derived. Acceptable short-term; if precise payment totals are
  required, extend the overview service or add
  `GET /v1/public/transactions/stats?period=`.
- ALB: existing rule covers `/v1/public/transactions*`.

## 3. Disputes — entire capability (MISSING)

- Page: Disputes & Errors
- UI features: Needs Response / Under Review counters, dispute table
  (Sub-Account, Type, Amount, Date Opened, Status), Submit-Evidence action.
- Owning service: Transactions
- Why missing: no disputes tables, protobuf, repositories, or routes exist
  anywhere in the Transactions service (verified by search).
- Proposed routes (all under existing `/v1/public/transactions*` ALB rule —
  no ALB change required):
  - `GET /v1/public/transactions/disputes/stats` →
    `{ needsResponse, underReview }`
  - `GET /v1/public/transactions/disputes?subAccountId=&status=&page=&pageSize=`
    → rows `{ id, subAccount, type, amount, dateOpened, status, dueIn }`
  - `POST /v1/public/transactions/disputes/{id}/evidence`
    `{ notes, documentIds[] }` (multipart or JSON)
- Authentication: Bearer token; add each path to the middleware
  protected-path list.
- Database: NEW tables required (`disputes`, `dispute_evidence`) — new
  migrations (up + down) + sqlc queries. Dispute lifecycle
  (NEEDS RESPONSE / Under Review / RESOLVED) must be modeled.
- Layers: proto → gateway regen → handlers → services → repositories →
  migrations → middleware.
- CORS: covered by existing shared middleware.
- Recommended sequence: migrations → sqlc → proto/regen → repo → service →
  handler → middleware entry → gateway tests.

## 4. Settings — team management (MISSING)

- Page: Settings
- UI features: team member list (User, Role, Status, Actions) and
  Invite Member.
- Owning service: Clients
- Why missing: Clients auth currently exposes only SignIn/RefreshToken/
  SignOut/CreateUser/UpdateUser (internal gRPC); there is no list-users,
  invite, or role-assignment HTTP/gRPC surface for dashboard use.
- Proposed routes (under existing `/v1/public/clients*` ALB rule — no ALB
  change required):
  - `GET /v1/public/clients/users?page=&pageSize=` →
    `{ rows: [{ id, email, name, role, status }], total, page, pageSize }`
  - `POST /v1/public/clients/users/invite`
    `{ email, role }` → creates pending invite (email delivery TBD)
  - `DELETE /v1/public/clients/users/{id}` (role-dependent authorization)
- Authentication: Bearer token, admin role required; add paths to the
  Clients middleware protected list (mirroring the auth middleware pattern in
  `transactions/auth/middleware.go`).
- Database: the existing `users` table (migration 000005) can support
  listing; invitations likely need a new `user_invites` table (up + down
  migration + sqlc).
- Layers: proto (ClientsService) → gateway regen → handler → service → repo
  → middleware → tests.

---

## ALB / CORS summary

- Every proposed route fits an EXISTING ALB listener rule
  (`/v1/public/transactions*`, `/v1/public/clients*`). **No ALB routing
  change and no CORS middleware change are required.** All preflights are
  already answered by the shared `shared/observability/cors.go` middleware
  (Allow-Headers: Content-Type, Authorization).

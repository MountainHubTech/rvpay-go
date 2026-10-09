# Missing Backend Endpoints — Transactions / Disputes / Settings Pages

Status snapshot updated 2026-09-10 after implementing all three dashboard
pages AND their backends. Items 1–3 and the users-list half of item 4 are now
IMPLEMENTED; only the invite/delete team-management actions remain. See the
per-service checkpoints for full details.

---

## 1. Transactions — transaction list (IMPLEMENTED)

- Page: Transactions
- UI feature: paginated table (Transaction ID, Sub-Account, Customer, Amount,
  Status, Gateway, Date) with sub-account/status filters.
- Owning service: Transactions
- HTTP route: `GET /v1/public/transactions` (live)
- gRPC: `ListTransactions` on `DashboardOverviewService` in
  `protobuf/transactions.proto`
- HTTP method: GET
- Authentication: Bearer access token (protected-path list in
  `transactions/cmd/grpc-service/main.go`)
- Request params: `search`, `status`, `sub_account`, `page`, `page_size`
- Response: `{ rows: [{ id, short_id, sub_account, customer, customer_initials,
  amount, status, gateway, date }], total, page, page_size }`
- Database: deposits via `ListDepositsFiltered`/`CountDepositsFiltered`
  (status normalization Success/Failed/Pending; no Refunded deposit concept).
- ALB: covered by existing rule (`/v1/public/transactions*`).
- Dashboard wiring: `fetchTransactions` + `transactions/page.tsx` (DONE).

## 2. Transactions — transaction statistics refinement (PARTIALLY EXISTS)

- Page: Transactions
- UI feature: "Total Pending" and "Total Cleared (30d)" stat cards.
- Currently satisfied by `GET /v1/public/transactions/overview/snapshot`
  (`pendingPayouts`, `totalRevenue`) — payout-derived, not
  transaction-derived. Acceptable short-term; if precise payment totals are
  required, extend the overview service or add
  `GET /v1/public/transactions/stats?period=`.
- ALB: existing rule covers `/v1/public/transactions*`.

## 3. Disputes — entire capability (IMPLEMENTED)

- Page: Disputes & Errors
- UI features: Needs Response / Under Review counters, dispute table
  (Sub-Account, Type, Amount, Date Opened, Status), Submit-Evidence action.
- Owning service: Transactions
- Live routes (all under existing `/v1/public/transactions*` ALB rule — no
  ALB change required):
  - `GET /v1/public/transactions/disputes/stats` →
    `{ needsResponse, underReview }`
  - `GET /v1/public/transactions/disputes?search=&status=&page=&page_size=`
    → rows `{ id, subAccount, type, amount, dateOpened, status, dueIn }`
  - `POST /v1/public/transactions/disputes/evidence` `{ id, notes }` →
    `{ dispute }` (static path so the transport middleware matches exactly)
- Authentication: Bearer admin token; each path added to the middleware
  protected-path list.
- Database: `disputes` table + `dispute_status` enum (NEEDS_RESPONSE /
  UNDER_REVIEW / RESOLVED) via migration 000005 (up + down) + sqlc queries.
- Layers: proto → gateway regen → repo → service → middleware (DONE).
- Dashboard wiring: `fetchDisputes`/`fetchDisputeStats` + `disputes/page.tsx`
  (DONE).

## 4. Settings — team management (PARTIAL — list IMPLEMENTED, invite/delete REMAIN)

- Page: Settings
- UI features: team member list (User, Role, Status, Actions) and
  Invite Member.
- Owning service: Clients
- IMPLEMENTED: `GET /v1/public/clients/users?search=&role=&page=&page_size=`
  → `{ rows: [{ id, name, email, role, status, dateJoined }], total, page,
  page_size }` via `AuthService.ListUsers`; wired to
  `fetchUsers` + `settings/page.tsx` team table.
- REMAINS: `POST /v1/public/clients/users/invite` `{ email, role }` (pending
  invite; email delivery TBD) and `DELETE /v1/public/clients/users/{id}`
  (role-dependent authorization). The dashboard keeps the Invite action
  disabled until the invite endpoint lands.
- Database: the existing `users` table (migration 000005) supports listing;
  invitations require a new `user_invites` table (up + down migration + sqlc).

---

## ALB / CORS summary

- Every proposed route fits an EXISTING ALB listener rule
  (`/v1/public/transactions*`, `/v1/public/clients*`). **No ALB routing
  change and no CORS middleware change are required.** All preflights are
  already answered by the shared `shared/observability/cors.go` middleware
  (Allow-Headers: Content-Type, Authorization).

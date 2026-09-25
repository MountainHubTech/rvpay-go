# Cline Agent — Server Connection / Admin Dashboard ↔ RVPay Services

## Mission

You are the **Admin Dashboard Server-Connection Mapping Agent**.

Inspect the entire current `admindashboard/`, `clients/`, and `transactions/` implementation and determine exactly how the Admin Dashboard must communicate with the existing RVPay server/services.

The objective is to make the Admin Dashboard obtain everything it needs from the server while preserving existing service contracts wherever they already satisfy the dashboard.

You are primarily a **contract-mapping, gap-identification, and dashboard-adaptation agent**.

---

## 1. Mandatory reading

Before changing anything, read ALL of these files.

### `admindashboard/`
- `.clinecheck.md`
- `.clineignore.md`
- `.clinerules.md`
- `.dashboard-setup.md`
- `.project-checkpoint.md`
- `.project-context.md`
- `README.md`

Also inspect the actual dashboard implementation, especially:
- `lib/dashboard-data.ts`
- `app/**`
- `components/**`
- every `fetch(...)`
- API helpers/server actions, if any
- TypeScript interfaces/types used by dashboard components

Do not rely only on `.dashboard-setup.md`; verify the actual code.

### `clients/`
- `.clinecheck.md`
- `.clineignore.md`
- `.clinerules.md`
- `.dashboard-setup.md`
- `.service-checkpoint.md`
- `README.md`

Also inspect relevant HTTP handlers/routes, services, repositories, SQL/query definitions, protobuf/gateway contracts, and tests.

Pay particular attention to the post-Agent-02/03/04/05/06 sections of `.service-checkpoint.md`.

### `transactions/`
- `.clinecheck.md`
- `.clineignore.md`
- `.clinerules.md`
- `.dashboard-setup.md`
- `.service-checkpoint.md`
- `.service-context.md`
- `README.md`

Also inspect:
- protobuf definitions
- grpc-gateway mappings
- HTTP routes
- deposits
- payouts
- repositories/queries
- VerifyPayment
- PawaPay integration
- deposit callback
- tests

Treat the current checkpoint's post-September-2 changes as authoritative unless the actual code proves otherwise.

---

## 2. Core rules

### Rule A — Existing endpoint first

For every dashboard requirement:

1. Identify the dashboard's exact request.
2. Identify its required response.
3. Search BOTH services for an existing endpoint.
4. Compare method, path, query parameters, body, response, status semantics, identifiers, pagination, filtering and sorting.
5. Only declare a new endpoint necessary when an existing endpoint genuinely cannot satisfy the requirement.

Do not create duplicate endpoints.

### Rule B — Wrong request body means FIX THE DASHBOARD

If an existing server endpoint is semantically correct but the dashboard sends the wrong body, **change the Admin Dashboard request body**.

Do NOT change the service merely to accommodate a malformed dashboard request.

For the existing deposit endpoint, preserve the established contract:

```text
POST /v1/public/deposits

clientName       = highlevel-<locationId>
customerId       = payment_initiate_props.contact.id
merchantId       = payer phone number (temporary/current backend requirement)
ghlTransactionId = genuine HighLevel transactionId
amount
paymentType
payerPhoneNumber
provider
```

Existing mappings include:
- `FCFA -> XAF`
- `mtn -> PROVIDER_MTN_MOMO`
- `orange -> PROVIDER_ORANGE_MOMO`

Never fabricate required identifiers.

`buyNowProductId` must NOT be treated as a HighLevel transactionId.

### Rule C — Response enrichment is additive

If an existing service endpoint is correct but its response lacks dashboard data:

- ADD fields.
- Preserve every existing field.
- Do not remove, rename, or break existing fields.
- Prefer backward-compatible additive response enrichment.

### Rule D — Preserve working service behavior

Do not unnecessarily alter:
- PawaPay initiation/status behavior
- ACCEPTED/REJECTED/unknown status handling
- XAF zero-decimal behavior
- `ghl_transaction_id` correlation
- VerifyPayment GET contract
- HighLevel single OAuth exchange
- locationId tenancy mapping
- INSTALL provisioning/idempotency
- provider capability/association/configuration sequencing

Only make backend changes directly necessary for dashboard data.

---

## 3. Dashboard mapping requirements

Inspect every active page and map its actual request/response needs.

### `/` — Overview

Current documented seam:

```text
GET /api/v1/overview/snapshot?period=
```

Expected conceptual shape:

```json
{
  "statCards": [],
  "revenueOverTime": [],
  "needsAttention": [],
  "recentPayouts": []
}
```

Verify the exact TypeScript shapes in `lib/dashboard-data.ts`.

Supported periods currently documented:
- `Last 7 Days`
- `Last 30 Days`
- `Last 90 Days`
- `This Year`

The selected period must reach the backend.

Determine whether existing Transactions/Clients data can provide the snapshot. Prefer one clean endpoint rather than four unnecessary endpoints.

### `/sub-accounts`

Documented request:

```text
GET /api/v1/sub-accounts
  ?search=
  &status=Active|Restricted|Inactive|All
  &sort=dateJoined
  &order=asc|desc
  &page=
  &pageSize=
```

Expected row data includes:

```text
id
name
location
initials
avatarClass OR sufficient raw data to derive it
status
balance
lastPayoutDate
totalProcessed
```

Add Account:

```text
POST /api/v1/sub-accounts
```

Inspect the Clients service before creating anything.

Preserve:
- HighLevel client naming `highlevel-<locationId>`
- `external_account_id = locationId`
- idempotency
- platform lookup by `slug="highlevel"`
- no platform creation
- distinction between companyId and locationId

If an existing endpoint matches but the dashboard body is wrong, fix the dashboard.

### `/payouts`

Documented requests:

```text
GET /api/v1/payouts/overview/stats
```

and:

```text
GET /api/v1/payouts
  ?search=
  &status=Failed|In Transit|Cleared|Pending
  &page=
  &pageSize=
```

Expected list response:

```json
{
  "rows": [
    {
      "id": "...",
      "name": "...",
      "location": "...",
      "initials": "...",
      "avatarClass": "...",
      "amount": "...",
      "status": "Failed|In Transit|Cleared|Pending",
      "initiated": "...",
      "expectedOrCleared": "...",
      "expectedOrClearedStrong": true
    }
  ],
  "total": 0,
  "page": 1,
  "pageSize": 20
}
```

Also currently planned:

```text
GET /api/v1/payouts/export
GET /api/v1/payouts/{id}
```

for CSV export and row expansion.

Inspect the existing Transactions payout API before declaring new endpoints.

If the existing payout response lacks dashboard fields, enrich it additively.

### `/transactions`

The documented future request is:

```text
GET /api/v1/transactions?search=&status=&page=&pageSize=
```

Do NOT implement merely because it appears in the dashboard setup.

If the actual page is still a placeholder, record it as not currently required.

If it is active, determine whether an existing Transactions endpoint can satisfy it.

### `/disputes`

Future request:

```text
GET /api/v1/disputes?status=&page=&pageSize=
```

Do not invent a dispute model. Only act if the actual dashboard requires it.

### `/settings`

Future requests:

```text
GET /api/v1/settings
PUT /api/v1/settings
```

Only act if the actual page is functional.

### `/me`

The dashboard documents:

```text
GET /me
```

with:

```json
{
  "name": "...",
  "email": "...",
  "avatarUrl": "..."
}
```

Determine whether an existing identity endpoint exists. If not, document the missing endpoint rather than inventing authentication.

### Notifications

The notification bell is currently optional/inert. Do not create an endpoint unless the actual dashboard now calls one.

---

## 4. `/payment` is already live

Preserve:

```text
POST https://api.rvpay.xyz/v1/public/deposits
GET  https://api.rvpay.xyz/v1/public/payments/verify
```

Verify the actual dashboard request against the current Transactions protobuf/gateway implementation.

Verify VerifyPayment query parameters:

```text
ghlTransactionId
ghlChargeId
subscriptionId
```

The existing VerifyPayment response includes:

```text
success
failed
message
```

If the payment UI needs more information, ADD fields without removing these.

Do not replace the public payment API with an invented dashboard-specific API.

---

## 5. Clients service constraints

Preserve the current HighLevel flow:

```text
GHL install
 -> OAuth callback
 -> one code exchange
 -> tokenResp.LocationID
 -> client highlevel-<locationId>
 -> integration external_account_id=locationId
 -> OAuth token
 -> capabilities
 -> provider association
 -> base-config GET verification
 -> optional credentials POST
 -> local provider persistence
```

Never expose to the dashboard:
- access tokens
- refresh tokens
- client secrets
- provider API keys
- OAuth authorization codes
- DB credentials

Do not create the HighLevel platform.

Do not confuse company/agency IDs with location/sub-account IDs.

---

## 6. Transactions service constraints

Preserve the current facts:

- `pawapay_client` is already bumped and legacy deposits were aligned.
- repository-wide build/tests were restored.
- HighLevel external deposit identifiers are implemented.
- `ghl_transaction_id` is persisted and used by VerifyPayment.
- VerifyPayment is GET.
- InitiateDeposit uses one DB transaction around INSERT + PawaPay initiation.
- Only PawaPay `ACCEPTED` commits.
- REJECTED and unexpected statuses roll back.
- XAF is serialized as a whole number; fractional XAF is rejected.
- PawaPay callback exists at:

```text
POST /v1/public/pawapay/deposits/callback
```

Do not alter these for dashboard convenience.

---

## 7. Required internal mapping

Before editing, create a complete internal mapping table:

| Dashboard feature | Request | Existing endpoint | Request compatible? | Response sufficient? | Action |
|---|---|---|---|---|---|
| Overview | ... | ... | YES/NO | YES/NO | ... |
| Sub-accounts | ... | ... | YES/NO | YES/NO | ... |
| Payout stats | ... | ... | YES/NO | YES/NO | ... |
| Payout list | ... | ... | YES/NO | YES/NO | ... |
| Payment initiation | ... | ... | YES/NO | YES/NO | ... |
| Payment verification | ... | ... | YES/NO | YES/NO | ... |

Classify every finding as one of:

### A — Dashboard-only change
Existing server contract is correct; dashboard request is wrong.

### B — Additive service response change
Existing endpoint is correct; response needs additional fields.

### C — New backend endpoint
No existing endpoint can satisfy the requirement.

### D — Not currently required
Dashboard feature is still placeholder/inert.

---

## 8. New endpoint delegation

If a new endpoint is genuinely required, DO NOT implement it in this mapping task unless the repository rules explicitly require implementation.

Instead append a precise next-agent task to the appropriate service checkpoint.

For every new endpoint document:

```text
Endpoint:
HTTP method:
Service:
Purpose:
Dashboard caller:
Query parameters:
Request body:
Response body:
Required source data:
Pagination:
Filtering:
Sorting:
Authorization requirement:
Existing repositories/data that can support it:
Why existing endpoints cannot satisfy it:
Tests required:
```

Do not call something "new" merely because its URL differs from the dashboard's suggested URL. Determine whether the existing endpoint can be called directly or whether a true capability gap exists.

---

## 9. Checkpoint updates

APPEND — never overwrite — these files:

### Admin Dashboard
```text
admindashboard/.clinecheck.md
admindashboard/.project-checkpoint.md
```

Include:
- agent name/date/status
- complete dashboard-to-server mapping
- dashboard changes
- endpoint decisions
- response enrichments requested
- delegated new endpoints
- verification
- remaining gaps

### Clients
```text
clients/.clinecheck.md
clients/.service-checkpoint.md
```

Append a section:

```text
## Admin Dashboard Server-Connection Mapping — Clients
```

Include only Clients findings:
- reused endpoints
- additive response fields
- genuinely missing endpoints
- exact next-agent implementation tasks
- tests required

### Transactions
```text
transactions/.clinecheck.md
transactions/.service-checkpoint.md
```

Append:

```text
## Admin Dashboard Server-Connection Mapping — Transactions
```

Include only Transactions findings:
- reused endpoints
- dashboard request corrections
- additive response fields
- genuinely missing endpoints
- exact next-agent implementation tasks
- tests required

Never rewrite previous Agent sections.

---

## 10. Exact delegation example

Use this level of specificity:

```text
### Next Agent Required — Transactions

1. Add dashboard read endpoint:

   GET /api/v1/payouts
   Query:
     search
     status
     page
     pageSize

   Response must preserve existing payout fields and additionally expose:
     ...

   Reason:
   No current endpoint provides paginated payout activity with the required
   account metadata and total count.

2. Add:

   GET /api/v1/payouts/overview/stats

   Response:
     ...
```

Never write vague statements such as "dashboard needs more payout data."

---

## 11. No fake data

Do not fabricate:
- transaction counts
- balances
- payout dates
- dispute records
- client IDs
- customer IDs
- HighLevel transaction IDs
- UUIDs
- provider information

If the server lacks required information, document the gap.

---

## 12. Authentication and API base URL

The dashboard currently has no authentication mechanism.

Do not invent a complete auth architecture during this mapping task unless existing project rules require it.

Document the current auth gap and identify privileged endpoints.

The dashboard currently hardcodes:

```text
https://api.rvpay.xyz
```

Do not casually introduce a new environment variable. If one is required, follow the dashboard rules and document it.

Do not modify deployment infrastructure.

---

## 13. Generated code

If a response change requires protobuf/gateway regeneration:

- modify the source protobuf
- use the repository's established generation process
- never hand-edit generated code

Do not change unrelated generated/dependency trees.

---

## 14. Testing

For dashboard changes, discover the actual scripts from `package.json` and run applicable:

```text
npm test
npm run lint
npm run build
```

Verify:
- TypeScript
- request bodies
- query parameters
- response parsing
- existing payment flow

For Clients changes:

```text
go test ./clients/... -count=1
go vet ./clients/...
```

For Transactions changes:

```text
go test ./transactions/... -count=1
go vet ./transactions/...
```

Do not fix unrelated failures.

---

## 15. Final report

At completion report:

### Dashboard endpoints usable immediately
List every endpoint already satisfying a dashboard requirement.

### Dashboard changes made
List every request body, URL, query parameter, or response-consumption change made in `admindashboard/`.

### Service response enrichments required
Separate Clients and Transactions.

For each:
- endpoint
- existing response
- added fields
- reason
- backward-compatible approach

### New endpoints required
Separate Clients and Transactions.

For each provide the full implementation contract.

### Not currently required
List placeholder/inert features that do not currently require backend work.

### Remaining blockers
Only real blockers.

### Verification
Report exact commands and results.

---

## 16. Success criteria

This agent succeeds only when:

1. Every active Admin Dashboard network request has a known server destination.
2. Every request body matches the correct existing server contract.
3. Existing endpoints are reused wherever semantically possible.
4. Dashboard request mistakes are fixed in the dashboard rather than distorting services.
5. Missing response data is handled additively.
6. Truly missing backend capabilities are documented precisely for the next service agents.
7. Existing payment/OAuth behavior remains intact.
8. All six requested checkpoint files are appended to, never overwritten.
9. The next Clients and Transactions agents can implement their work directly from the checkpoint entries.
10. No fake production data or fabricated identifiers are introduced.

Do not declare completion until the actual source code has been inspected and the mapping has been verified against the current implementation.

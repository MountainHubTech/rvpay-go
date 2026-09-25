# Cline Agent — Server Connection Implementation + Transactional Customer Flow

## Mission

Continue from the completed `server-connection` audit/contract-mapping task.

The objective of this task is now **implementation**:

1. Make the Admin Dashboard's currently required server data endpoints actually available.
2. Make sure the Admin Dashboard consumes those endpoints correctly.
3. Update `admindashboard/.dashboard-setup.md` so it accurately describes which endpoints are LIVE and which are FUTURE/NOT LIVE.
4. Complete the Customer flow so Customer creation is part of the **same database transaction as InitiateDeposit + PawaPay initiation**.
5. Add the Customer information to the InitiateDeposit request and update `admindashboard/app/payment/page.tsx`.
6. Change the Customer database association from `client_id` to `client_name` through a **new migration**, propagating the change through DB → persistence → service → protobuf/presentation → dashboard.
7. Append implementation results to the existing checkpoints without overwriting previous agent history.

This is an implementation task, not another audit.

---

# 1. REQUIRED CONTEXT

Before making any changes, consume the existing project context.

## Admin Dashboard

Read:

```text
admindashboard/.clinecheck.md
admindashboard/.clineignore.md
admindashboard/.clinerules.md
admindashboard/.dashboard-setup.md
admindashboard/.project-checkpoint.md
admindashboard/.project-context.md
admindashboard/README.md
```

Then inspect the actual dashboard source, especially:

```text
admindashboard/app/
admindashboard/components/
admindashboard/lib/
admindashboard/package.json
admindashboard/app/payment/page.tsx
```

Search all dashboard requests.

Do NOT rely solely on the previous checkpoint's description of the UI. Confirm what the current source actually renders and requests.

---

## Clients

Read:

```text
clients/.clinecheck.md
clients/.clineignore.md
clients/.clinerules.md
clients/.dashboard-setup.md
clients/.service-checkpoint.md
clients/README.md
```

Then inspect the actual relevant Clients implementation.

Pay particular attention to:

```text
clients/cmd/grpc-service/main.go
clients/http/
clients/service/
clients/providers/
clients/oauth/
clients/webhooks/
clients/db/
clients/db/query/
clients/db/repo/
clients/*.proto
```

Only modify Clients if the implementation genuinely requires it.

---

## Transactions

Read:

```text
transactions/.clinecheck.md
transactions/.clineignore.md
transactions/.clinerules.md
transactions/.dashboard-setup.md
transactions/.service-checkpoint.md
transactions/.service-context.md
transactions/README.md
```

Then inspect the actual Transactions implementation, especially:

```text
transactions/deposits/
transactions/payouts/
transactions/customers/
transactions/db/
transactions/db/query/
transactions/db/repo/
transactions/db/migrations/
transactions/proto/
transactions/gateway/
transactions/cmd/grpc-service/
```

Search the entire repository for:

```text
customer_id
client_id
customers
Customer
CreateCustomer
GetCustomer
InitiateDeposit
CreateDepositRequest
ghl_transaction_id
```

---

# 2. IMPORTANT IMPLEMENTATION RULES

These rules are mandatory.

## Rule A — Existing endpoint first

For every Dashboard requirement:

1. Find an existing server endpoint.
2. Compare:

   * HTTP method
   * path
   * query parameters
   * request body
   * response body
3. If the endpoint already provides the required capability but the Dashboard sends the wrong request body:

   * **FIX THE ADMIN DASHBOARD**
   * DO NOT change the service merely to accommodate an incorrect Dashboard request.
4. If the service response does not contain enough information:

   * ADD fields to the response.
   * Do NOT remove or rename existing response fields.
5. Only create a new endpoint where there is a genuine missing capability.

## Rule B — No fabricated data

Do not fabricate:

* IDs
* balances
* dates
* transactions
* customers
* payout values
* revenue
* status
* HighLevel identifiers

Everything shown by the Dashboard must come from actual service/database data.

## Rule C — Preserve existing behavior

Do not regress:

* HighLevel OAuth
* HighLevel Marketplace installation
* provider registration
* PawaPay integration
* VerifyPayment GET contract
* `ghl_transaction_id`
* ACCEPTED-only PawaPay commit behavior
* REJECTED rollback behavior
* unexpected-status rollback
* XAF whole-number handling
* fractional-XAF rejection
* PawaPay callback correlation

## Rule D — Additive response changes

If a response needs more information for the Dashboard:

```text
ADD
```

Do not remove existing response fields.

Existing clients must remain compatible.

## Rule E — Never edit old migrations

If the Customer schema must change:

```text
CREATE A NEW MIGRATION
```

Do not modify an already-applied migration.

## Rule F — Never hand-edit generated code

Modify the source:

* proto
* SQL
* sqlc configuration

then regenerate according to the repository's established commands.

---

# 3. SERVER CONNECTION IMPLEMENTATION

The previous agent determined that the Dashboard's immediate live payment calls are:

```text
POST /v1/public/deposits
GET  /v1/public/payments/verify
```

It also identified these missing Dashboard data capabilities:

```text
GET  /api/v1/overview/snapshot?period=
GET  /api/v1/payouts/overview/stats
GET  /api/v1/payouts?search=&status=&page=&pageSize=
GET  /api/v1/sub-accounts?search=&status=&sort=&order=&page=&pageSize=
POST /api/v1/sub-accounts
```

These were previously delegated because the Dashboard will eventually require them.

Now implement the endpoints that the **current Dashboard actually needs**.

Do not implement an endpoint merely because it appears in the old checkpoint.

---

# 4. OVERVIEW

Implement:

```http
GET /api/v1/overview/snapshot?period=
```

Supported periods should correspond to the Dashboard:

```text
7d
30d
90d
ytd
```

Inspect the Dashboard's actual period values before choosing the final wire representation.

Return all information actually rendered by the Overview page.

The conceptual response is:

```json
{
  "statCards": [],
  "revenueOverTime": [],
  "needsAttention": [],
  "recentPayouts": []
}
```

The actual field names/types must follow the existing Dashboard conventions.

Do not fabricate values.

If the underlying services do not currently expose sufficient data, implement the smallest legitimate server-side capability required and document any remaining cross-service dependency.

---

# 5. PAYOUT OVERVIEW

Implement:

```http
GET /api/v1/payouts/overview/stats
```

Return the real metrics required by the Dashboard.

Inspect the actual Dashboard code to determine:

* metric names
* numeric types
* date/time requirements
* status breakdowns
* currency handling

Do not invent additional metrics merely because they sound useful.

---

# 6. PAYOUT LIST

Implement:

```http
GET /api/v1/payouts?search=&status=&page=&pageSize=
```

The endpoint must support:

* search
* status filtering
* pagination
* deterministic ordering
* total count
* Dashboard-required payout fields

Conceptually:

```json
{
  "rows": [],
  "total": 0,
  "page": 1,
  "pageSize": 20
}
```

Use actual existing payout data.

Preserve existing payout response structures where applicable.

If the Dashboard expects fields missing from the current response, ADD them.

Do not remove existing fields.

Do not implement payout export/detail unless the current Dashboard actually requests them.

---

# 7. SUB-ACCOUNTS

Implement:

```http
GET /api/v1/sub-accounts?search=&status=&sort=&order=&page=&pageSize=
```

and:

```http
POST /api/v1/sub-accounts
```

ONLY if the current Add Account UI actually performs the POST.

The implementation must preserve the existing HighLevel tenancy model:

```text
client name:
highlevel-<locationId>

integration.external_account_id:
locationId

platform:
slug = "highlevel"
```

Never:

```text
companyId == locationId
```

Never create the HighLevel platform.

Resolve it using:

```text
slug = "highlevel"
```

Preserve idempotency.

Do not fabricate:

* balance
* lastPayoutDate
* totalProcessed
* transaction counts

If these require Transactions-owned information, follow the existing service architecture rather than creating direct database coupling between services.

If some Dashboard field cannot yet be legitimately populated, document it in the appropriate checkpoint.

---

# 8. INACTIVE DASHBOARD FEATURES

Inspect the actual Dashboard source for:

```text
transactions
disputes
settings
/me
payout export
payout detail
notifications
topbar search
```

If these are static/placeholder and make no network request:

DO NOT implement them in this task.

Document them as:

```text
FUTURE / NOT LIVE
```

in:

```text
admindashboard/.dashboard-setup.md
```

Do not invent an authentication architecture.

---

# 9. EXISTING PAYMENT CONTRACT

Do not regress the existing:

```http
POST /v1/public/deposits
```

The Dashboard currently sends:

```text
clientName = highlevel-<locationId>
customerId = contact.id
merchantId = payer phone number
ghlTransactionId = genuine HighLevel transactionId
amount
paymentType
payerPhoneNumber
provider
```

Provider mapping:

```text
mtn    -> PROVIDER_MTN_MOMO
orange -> PROVIDER_ORANGE_MOMO
```

The Dashboard must continue to send the genuine HighLevel transaction identifier.

Do NOT use:

```text
buyNowProductId
```

as a transaction ID.

Also preserve:

```http
GET /v1/public/payments/verify?ghlTransactionId=&ghlChargeId=&subscriptionId=
```

Do not remove:

```text
success
failed
message
```

or any existing response fields.

---

# 10. CUSTOMER FLOW — MANDATORY

This is the second major workstream.

The current InitiateDeposit flow must be upgraded so that Customer creation occurs inside the same DB transaction as the Deposit and PawaPay initiation.

The desired transaction is:

```text
BEGIN
    ↓
Create / resolve Customer
    ↓
Create Deposit
    ↓
Initiate PawaPay Deposit
    ↓
PawaPay status == ACCEPTED?
    ├── NO → ROLLBACK
    └── YES
          ↓
        COMMIT
```

This means:

```text
Customer
+
Deposit
```

must succeed or fail together.

---

# 11. CUSTOMER DATABASE MIGRATION

Inspect the current `customers` table and existing migration history.

The requested schema direction is:

OLD:

```text
client_id
```

NEW:

```text
client_name
name
address
```

Create a NEW migration.

Do not edit an existing migration.

Before dropping `client_id`, determine how existing customer rows are associated with clients.

Preserve existing data.

The migration must safely transform existing records where possible.

Important:

The checkpoint states that `clients.client_name` is not currently unique.

Therefore:

DO NOT blindly create:

```text
FOREIGN KEY customer.client_name -> clients.client_name
```

unless you first establish that uniqueness is guaranteed and adding it is safe.

Prefer application/repository-level client scoping if a database FK cannot safely be introduced.

Document the decision.

---

# 12. CUSTOMER DATA MODEL

The Customer representation must contain at minimum:

```text
clientName
name
phoneNumber
address
```

Use the repository's existing naming conventions.

Inspect the existing Customer proto/model before deciding exact capitalization/wire names.

The Customer must be identifiable by:

```text
client_name
```

and contain the supplied customer information.

Do not invent a new arbitrary natural key unless existing business logic requires it.

---

# 13. INITIATE DEPOSIT REQUEST

Extend the InitiateDeposit request to contain Customer information.

Conceptually:

```json
{
  "clientName": "highlevel-location123",
  "customerId": "ghl-contact-id",
  "customer": {
    "name": "...",
    "phoneNumber": "...",
    "address": "..."
  },
  "merchantId": "...",
  "ghlTransactionId": "...",
  "amount": {
    "amount": "...",
    "currency": "XAF"
  },
  "paymentType": "PAYMENT_TYPE_MMO",
  "payerPhoneNumber": "...",
  "provider": "PROVIDER_MTN_MOMO"
}
```

Use the repository's actual proto conventions.

The existing fields must remain compatible unless a field is explicitly being replaced by the requested Customer representation.

---

# 14. ADMIN DASHBOARD PAYMENT PAGE

Update:

```text
admindashboard/app/payment/page.tsx
```

The payment request must include Customer information:

```text
name
phoneNumber
address
```

Use the actual customer/contact information available to the page.

Do not fabricate values.

Do not use:

```text
buyNowProductId
```

as a transaction ID.

Continue passing:

```text
clientName
customerId
merchantId
ghlTransactionId
payerPhoneNumber
provider
amount
paymentType
```

as required by the existing contract.

---

# 15. CUSTOMER PERSISTENCE

Propagate the schema change through:

```text
DB migration
    ↓
SQL
    ↓
sqlc generation
    ↓
repository
    ↓
transaction-aware repository
    ↓
service
    ↓
protobuf
    ↓
gateway / HTTP presentation
    ↓
Admin Dashboard
```

Inspect the existing repository transaction mechanism.

Reuse it.

Do NOT introduce a new transaction framework.

---

# 16. CUSTOMER CREATION SEMANTICS

Inspect existing Customer repository/service behavior before deciding whether:

```text
CreateCustomer
```

or:

```text
GetOrCreateCustomer
```

is appropriate.

Avoid blindly inserting duplicate customers for every payment.

Use existing constraints/business rules where they exist.

If there is no existing natural key, define the smallest safe lookup/scoping behavior supported by the existing schema.

The customer must remain associated with:

```text
client_name
```

---

# 17. TRANSACTIONAL INITIATE DEPOSIT

The InitiateDeposit service must become:

```text
BEGIN
    ↓
customer create/reuse
    ↓
deposit INSERT
    ↓
PawaPay InitiateDeposit
    ↓
validate PawaPay response
    ↓
COMMIT
```

Existing PawaPay behavior must remain:

```text
ACCEPTED → commit
REJECTED → rollback + Internal
unexpected status → rollback + Internal
HTTP/transport failure → rollback + Internal
```

Customer creation must roll back in all of those failure cases.

---

# 18. REQUIRED ROLLBACK TESTS

Add tests for:

### Test 1

```text
Customer + Deposit + PawaPay ACCEPTED
```

Expected:

```text
Customer exists
Deposit exists
transaction committed
```

### Test 2

```text
PawaPay HTTP 500
```

Expected:

```text
Customer does not exist
Deposit does not exist
```

### Test 3

```text
PawaPay REJECTED
```

Expected:

```text
Customer does not exist
Deposit does not exist
```

### Test 4

```text
PawaPay unexpected status
```

Expected:

```text
Customer does not exist
Deposit does not exist
```

### Test 5

Customer creation failure.

Expected:

```text
Deposit is not committed
```

### Test 6

Deposit creation failure.

Expected:

```text
Customer is not committed
```

### Test 7

Commit failure.

Expected:

```text
Internal error
```

and correct rollback/error semantics according to the existing transaction abstraction.

### Test 8

Verify persisted Customer fields:

```text
client_name
name
phone_number
address
```

### Test 9

Verify correct client association:

```text
client_name = highlevel-<locationId>
```

Use `httptest.Server` for PawaPay.

Never contact the real PawaPay API from tests.

---

# 19. EXISTING PawaPay REQUIREMENTS TO PRESERVE

Do not change these unless required to make the Customer transaction work:

### Amount

XAF must remain whole-number:

```text
25.00 → "25"
```

Fractional XAF remains rejected.

Other currencies retain existing behavior.

### Provider

Preserve:

```text
MTN_MOMO -> MTN_MOMO_CMR
ORANGE_MOMO -> ORANGE_MOMO_CMR
```

### Phone

Preserve the existing MSISDN representation expected by PawaPay.

### Status

Only:

```text
ACCEPTED
```

allows commit.

### Callback

Preserve:

```text
POST /v1/public/pawapay/deposits/callback
```

and correlation via:

```text
deposits.id
```

Do not introduce an invented callback signature mechanism.

---

# 20. RESPONSE ENRICHMENT

If the Dashboard needs Customer information in a response:

ADD:

```text
name
phoneNumber
address
clientName
```

Do not remove:

```text
success
failed
message
```

or existing fields.

Never expose:

```text
access_token
refresh_token
client_secret
api_key
database credentials
```

---

# 21. DASHBOARD SETUP DOCUMENT

After implementation, update:

```text
admindashboard/.dashboard-setup.md
```

It must become the authoritative operational contract for the Dashboard.

Clearly classify endpoints as:

```text
LIVE
```

or:

```text
FUTURE / NOT LIVE
```

At minimum evaluate:

### LIVE candidates

```text
POST /v1/public/deposits

GET /v1/public/payments/verify

GET /api/v1/overview/snapshot

GET /api/v1/payouts/overview/stats

GET /api/v1/payouts

GET /api/v1/sub-accounts

POST /api/v1/sub-accounts
```

Only mark an endpoint LIVE after implementation and verification.

Document:

* HTTP method
* path
* query parameters
* request body
* response body
* customer data contract
* authentication limitations
* current deployment/live-host limitations

---

# 22. FUTURE ENDPOINTS

If the current Dashboard does not yet request them, document as future rather than implementing them:

```text
GET /api/v1/transactions
GET /api/v1/disputes
GET /api/v1/settings
PUT /api/v1/settings
GET /me
GET /api/v1/payouts/{id}
GET /api/v1/payouts/export
```

Only include an endpoint if inspection confirms it is actually a future requirement.

---

# 23. CHECKPOINT UPDATES

APPEND only.

Never overwrite previous records.

Update:

```text
admindashboard/.clinecheck.md
admindashboard/.project-checkpoint.md

transactions/.clinecheck.md
transactions/.service-checkpoint.md
```

If Clients is changed, also append to:

```text
clients/.clinecheck.md
clients/.service-checkpoint.md
```

If Clients is untouched, leave its checkpoint files untouched.

The appended sections must include:

```text
Date
Agent name
Implementation
API contracts
Database changes
Customer flow
Transaction semantics
Tests
Verification
Remaining risks
Future endpoints
```

---

# 24. SERVICE CHECKPOINT REQUIREMENT

The Transactions and Clients checkpoints must explicitly identify any Dashboard requirements that remain unimplemented.

For every missing endpoint use:

```text
Endpoint:
Reason:
Required request:
Required response:
Owning service:
Dependencies:
```

This is the handoff contract for the next agent.

Do not merely say:

```text
TODO
```

---

# 25. TESTING

Run Transactions:

```text
go test ./transactions/... -count=1
go vet ./transactions/...
```

Run Clients if modified:

```text
go test ./clients/... -count=1
go vet ./clients/...
```

Run formatting:

```text
gofmt
```

according to the repository's existing conventions.

Run Dashboard:

```text
npm run lint
npm run build
```

and relevant tests if present.

If Linux `node_modules` is incomplete and prevents lint/build, document the environmental limitation.

Do not alter dependencies merely to hide an environment problem.

---

# 26. PROTOBUF / SQLC

If proto changes are required:

1. modify source proto;
2. run the established protobuf generation command;
3. verify generated output;
4. never hand-edit generated files.

If SQL changes are required:

1. modify SQL;
2. create migration if schema changes;
3. regenerate sqlc;
4. inspect generated code;
5. run tests.

Never hand-edit:

```text
grpc/go/
db/sqlc/
```

generated output.

---

# 27. DO NOT TOUCH

Unless directly required by this task:

```text
infra/
CloudFormation
.env
secrets
legacy unrelated services
transactions unrelated to deposits/payouts/customers
HighLevel OAuth/provider logic
PawaPay SDK source
generated dependency trees
third_party/
```

Do not perform unrelated refactoring.

---

# 28. FINAL VERIFICATION MATRIX

Before declaring completion, produce this matrix:

| Requirement                    | Implemented | Tested |
| ------------------------------ | ----------: | -----: |
| Overview endpoint              |             |        |
| Payout stats endpoint          |             |        |
| Payout list endpoint           |             |        |
| Sub-account list               |             |        |
| Sub-account create             |             |        |
| Customer migration             |             |        |
| `client_id → client_name`      |             |        |
| Customer name persistence      |             |        |
| Customer phone persistence     |             |        |
| Customer address persistence   |             |        |
| InitiateDeposit Customer body  |             |        |
| Dashboard payment page         |             |        |
| Customer inside DB transaction |             |        |
| ACCEPTED commit                |             |        |
| PawaPay 500 rollback           |             |        |
| PawaPay REJECTED rollback      |             |        |
| Unexpected status rollback     |             |        |
| Customer creation failure      |             |        |
| Deposit creation failure       |             |        |
| VerifyPayment compatibility    |             |        |
| Dashboard setup updated        |             |        |
| Checkpoints appended           |             |        |

---

# 29. FINAL REPORT

At the end, report:

## A. LIVE Dashboard endpoints

List every endpoint that is actually live and verified.

For each:

```text
METHOD
PATH
Request
Response
Owning service
```

## B. Dashboard changes

List every Dashboard file changed.

Especially:

```text
app/payment/page.tsx
.dashboard-setup.md
```

## C. Customer migration

Explain:

```text
old schema
new schema
migration number
data preservation
FK decision
```

## D. Customer flow

Show:

```text
Dashboard
→ InitiateDeposit request
→ service
→ Customer
→ Deposit
→ PawaPay
→ commit/rollback
```

## E. Transaction guarantees

Explicitly state what happens for:

```text
ACCEPTED
REJECTED
500
unexpected status
customer failure
deposit failure
commit failure
```

## F. Remaining endpoints

Separate:

```text
LIVE
FUTURE
BLOCKED
```

## G. Remaining risks

Only list real unresolved risks.

## H. Verification

Give exact commands and results.

---

# SUCCESS CRITERIA

This task is complete only when:

1. The current Dashboard's required data endpoints actually exist and work.
2. Existing endpoint contracts are reused where possible.
3. Incorrect Dashboard request bodies are fixed in the Dashboard rather than changing correct services.
4. Missing response information is added without removing existing fields.
5. `admindashboard/.dashboard-setup.md` accurately identifies LIVE endpoints.
6. InitiateDeposit accepts Customer information.
7. `payment/page.tsx` sends Customer information.
8. Customer schema uses `client_name` instead of `client_id`.
9. A new migration performs the schema change safely.
10. Customer fields include:

    * name
    * phone_number
    * address
11. Customer changes propagate through:

    * DB
    * SQL
    * sqlc
    * repository
    * service
    * protobuf
    * HTTP/gateway
    * Dashboard
12. Customer creation occurs inside the same DB transaction as Deposit + PawaPay initiation.
13. PawaPay failure rolls back both Customer and Deposit.
14. Only ACCEPTED permits commit.
15. Existing VerifyPayment and callback behavior remains intact.
16. Tests cover the transactional rollback cases.
17. Previous checkpoint history is preserved.
18. New checkpoint information is appended.
19. Clients checkpoint is changed only if Clients actually changes.
20. No unrelated refactoring or infrastructure changes are made.

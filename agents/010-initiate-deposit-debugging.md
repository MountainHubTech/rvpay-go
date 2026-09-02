Cline, perform a targeted end-to-end correction to the RVPay payment initiation identifier mapping across:

* `admindashboard/`
* `transactions/`

This task changes how HighLevel payment-context identifiers are passed into the RVPay Transactions service.

---

# IMPORTANT — CURRENT DOCUMENTS HAVE CHANGED

Do NOT rely on previous versions, previous agent reports, previous assumptions, or previous implementations.

Before making ANY changes, re-read the CURRENT versions of all relevant files.

## ADMINDASHBOARD — MUST READ FIRST

Read:

* `admindashboard/.clinerules.md`
* `admindashboard/.clineignore.md`
* `admindashboard/.project-checkpoint.md`
* `admindashboard/.project-context.md`
* `admindashboard/README.md`
* `admindashboard/Makefile`
* the CURRENT `admindashboard/app/payment/page.tsx`
* any directly relevant payment/client/API code in `admindashboard`

Also inspect any files required to understand how the payment page is built and published.

## TRANSACTIONS — MUST READ FIRST

Read:

* `transactions/README.md`
* `transactions/.clinecheck.md`
* `transactions/.clineignore.md`
* `transactions/.clinerules.md`
* `transactions/.service-checkpoint.md`
* `transactions/.service-context.md`
* `transactions/Makefile`

Then inspect the complete Transactions deposit flow, including:

* protobuf definitions
* generated protobuf / gRPC / gateway code where applicable
* presentation/API handlers
* service layer
* repository interfaces
* repository implementations
* SQL queries
* SQL schema/migrations
* models/types
* tests
* validation
* deposit creation
* customer lookup
* merchant lookup
* client lookup
* any code that assumes `client_id`, `customer_id`, or `merchant_id` are UUIDs

You MUST adhere strictly to both:

* `admindashboard/.clinerules.md`
* `admindashboard/.clineignore.md`

and:

* `transactions/.clinerules.md`
* `transactions/.clineignore.md`

Do not modify ignored files.

Do not bypass the project's established architecture or workflow.

---

# OBJECTIVE

Correct the payment identifier mapping between HighLevel → Admin Dashboard → RVPay Transactions service.

The current frontend is successfully receiving HighLevel's payment initiation context.

The remaining problem is that the Transactions service still assumes the incoming identifiers are RVPay UUIDs.

That assumption is now incorrect for this payment flow.

The intended mapping is:

```text
HighLevel payment_initiate_props
        │
        ├── locationId
        ├── contact.id
        └── transactionId
        │
        ▼
RVPay Admin Dashboard
        │
        ├── client_name
        ├── customer_id
        └── merchant_id
        │
        ▼
RVPay Transactions service
```

The mapping MUST be:

```text
client_name  = HighLevel location/client name used by RVPay
customer_id  = HighLevel contact.id
merchant_id  = HighLevel transactionId
```

The payment fields remain:

```text
amount
currency
payment_type
payer_phone_number
provider
```

---

# 1. CLIENT IDENTIFIER

## CURRENT PROBLEM

The Transactions service currently expects:

```go
clientID, err := uuid.Parse(req.GetClientId())
if err != nil {
    return nil, status.Error(codes.InvalidArgument, "client_id must be a valid UUID")
}
```

This is no longer correct for this payment integration.

The frontend should NOT send an RVPay client UUID.

Instead, the payment page must send the existing HighLevel/RVPay client name.

The client name already exists conceptually in the backend as the combination of the HighLevel platform stub and the HighLevel `locationId`.

For this HighLevel integration, this follows the existing RVPay naming convention, e.g.:

```text
highlevel-xxxxxxxxxxxxx
```

DO NOT invent a different naming convention.

Inspect the existing repository for the established client-name/platform-stub/location-ID convention and reuse it exactly.

---

# 2. CUSTOMER IDENTIFIER

The customer identifier sent from the Admin Dashboard must be:

```text
payment_initiate_props.contact.id
```

This is the HighLevel contact ID.

It MUST NOT be passed through:

```go
uuid.Parse(...)
```

It MUST be treated as an external string identifier.

Do not convert it into a UUID.

Do not generate an RVPay UUID from it.

Do not hash it.

Do not substitute another identifier.

---

# 3. MERCHANT IDENTIFIER

For the CURRENT implementation, the value mapped to `merchant_id` is:

```text
payment_initiate_props.transactionId
```

This is intentional and temporary.

Therefore:

```text
merchant_id = GHL transactionId
```

The backend will eventually receive a proper merchant identifier as the architecture evolves.

For THIS task, however, preserve the requested mapping.

Do not reinterpret `transactionId` as an RVPay UUID.

Do not call:

```go
uuid.Parse(req.GetMerchantId())
```

Do not generate a UUID.

Do not substitute `orderId`.

Do not substitute `buyNowProductId`.

Do not fabricate any identifier.

---

# 4. IMPORTANT — PRESERVE THE EXISTING HIGHLEVEL HANDSHAKE FIX

The previous Admin Dashboard work established that HighLevel's custom-payment iframe works by:

1. registering the `message` listener
2. sending the appropriate ready message
3. retrying the ready message
4. parsing stringified message payloads
5. receiving `payment_initiate_props`

Do NOT undo that work.

The current working behavior must remain.

The Admin Dashboard should continue to receive:

```text
payment_initiate_props
```

including:

```text
transactionId
locationId
contact.id
amount
currency
```

Do not regress the iframe handshake.

Do not replace it with the old single object-form ready message.

Do not make payment initiation dependent on a broken handshake.

---

# 5. ADMIN DASHBOARD MAPPING

Modify the CURRENT:

```text
admindashboard/app/payment/page.tsx
```

so that the backend initiation request uses the actual HighLevel values.

The intended mapping is:

```text
GHL locationId
      ↓
RVPay client_name

GHL contact.id
      ↓
RVPay customer_id

GHL transactionId
      ↓
RVPay merchant_id
```

The frontend MUST NOT attempt to obtain RVPay UUIDs for these values.

---

# 6. CLIENT NAME CONSTRUCTION

Before implementing this, inspect the repository to determine the existing exact client-name convention.

The intended value is conceptually:

```text
highlevel-{locationId}
```

but you MUST verify the existing RVPay convention before hardcoding it.

If the repository already has a helper/function/convention for constructing a HighLevel client name, reuse it.

Do not duplicate business logic unnecessarily.

If the repository has an established platform stub such as:

```text
highlevel
```

and combines it with:

```text
locationId
```

then use that exact convention.

The resulting value must be the same identifier that the backend already recognizes as the RVPay client name.

---

# 7. ADMIN DASHBOARD REQUEST

The current request is approximately:

```ts
const body = {
  amount: {
    amount: amount.toFixed(2),
    currency: currencyToApi(country.currency),
  },
  paymentType: "PAYMENT_TYPE_MMO",
  payerPhoneNumber: `${country.dialCode}${phone}`,
  provider,
  clientId?,
  customerId?,
  merchantId?,
};
```

That mapping must be corrected to the new contract.

The resulting request must represent:

```text
client_name = HighLevel-derived RVPay client name
customer_id = GHL contact.id
merchant_id = GHL transactionId
```

The exact JSON field names MUST match the final protobuf/grpc-gateway contract after the Transactions service changes.

Do not merely rename variables locally while continuing to send the old API contract.

Trace the request all the way through the gateway/protobuf definitions.

---

# 8. DO NOT LOSE THE HIGHLEVEL CONTEXT

The frontend must obtain the values from the actual `payment_initiate_props` payload.

Expected relevant fields include:

```text
locationId
contact.id
transactionId
amount
currency
```

The implementation must NOT fabricate missing values.

If:

```text
locationId
```

is missing, fail safely.

If:

```text
contact.id
```

is missing, fail safely.

If:

```text
transactionId
```

is missing, fail safely.

Do not send fake values simply to satisfy the backend.

Use the existing URL fallback only where the current application already has a legitimate value.

---

# 9. TRANSACTIONS PROTO CONTRACT

Inspect:

```text
protobuf/transactions.proto
```

or wherever the current `CreateDepositRequest` definition lives.

The current contract is:

```proto
message CreateDepositRequest {

  string client_id = 1;

  string customer_id = 2;

  string merchant_id = 3;

  commongrpc.Money amount = 4;

  commongrpc.PaymentType payment_type = 5;

  string payer_phone_number = 6;

  commongrpc.Provider provider = 7;
}
```

The identifier semantics must now reflect the actual integration.

At minimum, the client identifier needs to become a client name rather than a UUID.

Determine whether the established project convention requires:

```proto
string client_name = 1;
```

or another exact naming.

Prefer an explicit field rename rather than leaving a misleading `client_id` name if the service now fundamentally expects a client name.

Similarly, retain the customer and merchant field names only if they are still the established API contract, but change their semantics/types from RVPay UUIDs to external string identifiers.

Do NOT change unrelated protobuf contracts.

---

# 10. GENERATED CODE

If this repository checks generated protobuf/gateway code into source control, regenerate it using the repository's established Makefile/protobuf workflow.

Do NOT manually edit generated code unless the project's established process explicitly requires it.

Inspect:

```text
transactions/Makefile
```

and the relevant protobuf generation configuration.

Use the existing project command.

Ensure all generated layers remain consistent with the modified protobuf.

---

# 11. REMOVE UUID PARSING FROM DEPOSIT INITIATION

The current service contains:

```go
clientID, err := uuid.Parse(req.GetClientId())
if err != nil {
    return nil, status.Error(codes.InvalidArgument, "client_id must be a valid UUID")
}

customerID, err := uuid.Parse(req.GetCustomerId())
if err != nil {
    return nil, status.Error(codes.InvalidArgument, "customer_id must be a valid UUID")
}

merchantID, err := uuid.Parse(req.GetMerchantId())
if err != nil {
    return nil, status.Error(codes.InvalidArgument, "merchant_id must be a valid UUID")
}
```

These UUID validations must be removed/replaced.

The new implementation must validate the identifiers as strings according to the actual business requirements.

For example:

```text
client_name must not be empty
customer_id may be nullable/optional according to the new persistence contract
merchant_id may be nullable/optional according to the new persistence contract
```

Do not blindly introduce new validation rules.

Inspect existing repository/service semantics first.

---

# 12. IMPORTANT — CUSTOMER/MERCHANT ARE NOW OPTIONAL AT DATABASE LEVEL

Create a NEW database migration.

Do NOT modify an existing migration.

Do NOT rewrite migration history.

The new migration must:

1. remove the UUID-specific type/constraint from:

```text
client_id
customer_id
merchant_id
```

2. change the columns to the appropriate string/text representation required by the application.

3. remove the `NOT NULL` requirement from:

```text
customer_id
merchant_id
```

because these values may be unavailable in some flows.

4. preserve appropriate nullability and existing behavior everywhere else.

5. preserve data safely.

6. preserve or correctly adjust indexes/foreign keys/check constraints that depend on the old UUID types.

Do not blindly drop constraints.

Inspect the existing schema and migrations first.

---

# 13. IMPORTANT — FOREIGN KEYS

This requires careful inspection.

The current code suggests these identifiers may currently participate in relational validation.

For example, the current service does:

```go
customerRepo.GetByClientAndMerchantAndPhone(
    ctx,
    clientID,
    merchantID,
    phoneNumber,
)
```

This assumes:

```text
clientID = UUID
merchantID = UUID
```

That assumption must be removed.

The repository query and persistence layer must be adjusted to the new identifier model.

DO NOT simply cast strings to UUIDs.

DO NOT use fake UUIDs.

DO NOT preserve a foreign key that is incompatible with the new external identifier model.

Determine the correct existing data lookup strategy from the current schema and repository implementation.

---

# 14. CUSTOMER LOOKUP

The current code validates the customer using:

```go
customerRepo.GetByClientAndMerchantAndPhone(
    ctx,
    clientID,
    merchantID,
    phoneNumber,
)
```

This must be reviewed carefully.

The new values are:

```text
client_name
customer_id = GHL contact.id
merchant_id = GHL transactionId
phone_number
```

The repository/service must be changed so the lookup makes sense under the new identifier model.

Do not keep a method whose name/types imply UUID semantics if the underlying identifiers are now strings.

Trace the lookup all the way down:

```text
service
  ↓
repository interface
  ↓
repository implementation
  ↓
SQL query
  ↓
database schema
```

Update all layers consistently.

---

# 15. DEPOSIT PERSISTENCE

The deposit creation currently resembles:

```go
s.depositRepo.Create(
    ctx,
    clientID,
    customer.ID,
    merchantID,
    amount,
    currency,
    paymentType,
    phoneNumber,
    provider,
    sqlc.DepositStatusINITIATED,
    uuid.New(),
)
```

This must be reconsidered because:

```text
clientID
customer.ID
merchantID
```

no longer represent the same things.

The persistence layer must correctly store:

```text
client_name
customer_id
merchant_id
```

as the new external identifiers.

Do not accidentally replace:

```text
customer_id = GHL contact.id
```

with an internally generated RVPay customer UUID.

Do not accidentally replace:

```text
merchant_id = GHL transactionId
```

with an RVPay merchant UUID.

The values requested in this task must survive the complete persistence path.

---

# 16. DO NOT CONFUSE INTERNAL IDS WITH EXTERNAL IDS

RVPay may still have internal UUIDs elsewhere in its domain.

That is fine.

Do NOT globally eliminate UUIDs from RVPay.

This task only changes the identifiers involved in this payment initiation contract:

```text
client
customer
merchant
```

where the HighLevel integration supplies external string identifiers.

Preserve internal UUIDs that are unrelated to these fields.

For example, a deposit's own primary key may remain:

```text
uuid
```

and idempotency keys may remain:

```text
uuid
```

unless the existing architecture dictates otherwise.

---

# 17. PAWAPAY FLOW

Do NOT modify the PawaPay integration unnecessarily.

The PawaPay initiation should continue to receive the values it actually needs:

```text
amount
currency
phone number
provider
deposit ID
```

The identifier changes in this task are primarily about:

```text
HighLevel → RVPay transaction/persistence correlation
```

Do not change PawaPay SDK behavior.

Do not change provider logic.

Do not redesign the payment architecture.

---

# 18. STATUS/POLLING CONTRACT

Preserve the existing payment status verification flow.

The frontend should still use the real HighLevel:

```text
transactionId
```

for the existing GHL-specific verification mechanism.

Do not replace it with:

```text
depositId
```

unless the existing backend contract explicitly requires that.

The current conceptual flow remains:

```text
GHL
  │
  │ payment_initiate_props
  │
  ├── locationId
  ├── contact.id
  └── transactionId
  │
  ▼
Admin Dashboard
  │
  │ POST /v1/public/deposits
  │
  │ client_name = highlevel-{locationId}
  │ customer_id = contact.id
  │ merchant_id = transactionId
  │
  ▼
Transactions service
  │
  ├── validate/store identifiers
  ├── create deposit
  └── initiate PawaPay
  │
  ▼
PawaPay
  │
  ▼
callback/status
  │
  ▼
RVPay
  │
  ▼
Admin Dashboard
  │
  │ GET /v1/public/payments/verify
  │
  ▼
payment status
```

---

# 19. DO NOT INVENT A NEW API

Do NOT create a new endpoint.

Continue using the existing:

```text
POST /v1/public/deposits
```

unless repository inspection proves that the existing API contract has already been replaced.

This task changes the contract behind the existing endpoint.

It does NOT require:

```text
/payments/custom-provider/initiate
```

or any other new endpoint.

---

# 20. BACKWARD COMPATIBILITY

Before changing the protobuf/API contract, inspect how widely `CreateDepositRequest` is used.

Search for:

```text
CreateDepositRequest
GetClientId
GetCustomerId
GetMerchantId
client_id
customer_id
merchant_id
InitiateDeposit
CreateDeposit
```

Identify:

* REST callers
* gRPC callers
* tests
* admin dashboard
* internal services
* scripts
* documentation
* generated gateway code

Do not silently break unrelated callers.

If compatibility requires a carefully scoped transition, follow the repository's existing API/versioning conventions.

Do NOT invent a compatibility architecture.

---

# 21. MIGRATION SAFETY

The migration must be reversible where the repository's migration convention supports down migrations.

Before writing it:

1. inspect the current table definition
2. inspect all migrations affecting these columns
3. inspect indexes
4. inspect foreign keys
5. inspect SQL queries generated by sqlc
6. determine whether UUID→TEXT conversion requires explicit casts
7. determine whether existing UUID values can be safely converted to text

The migration must preserve existing data.

Do not drop and recreate the table.

Do not delete production data.

Do not reset migrations.

Do not edit historical migration files.

---

# 22. SQLC / PERSISTENCE

If the project uses sqlc, inspect:

```text
sqlc.yaml
```

and the relevant query files.

After changing schema/query types:

* regenerate sqlc using the project's established command
* inspect generated types
* ensure nullable database columns map to the project's established nullable representation
* update repository/service code accordingly

Do not manually edit generated sqlc output if the project normally regenerates it.

---

# 23. TESTS

Update existing tests affected by the contract change.

At minimum test:

### Valid HighLevel-style identifiers

Example conceptually:

```text
client_name = highlevel-abc123
customer_id = ghl-contact-123
merchant_id = ghl-transaction-456
```

These MUST NOT produce UUID validation errors.

### Missing client name

Must fail appropriately.

### Missing optional customer

Must behave according to the new nullable contract.

### Missing optional merchant

Must behave according to the new nullable contract.

### Deposit creation

Verify that the exact external identifiers survive into persistence.

### Existing payment initiation

Verify:

```text
amount
currency
payment type
phone
provider
```

remain unchanged.

### No UUID parsing

There must be no remaining UUID parsing of these three external identifier fields in the deposit initiation path.

Search the entire relevant Transactions service after implementation to confirm.

---

# 24. ADMIN DASHBOARD BUILD

After modifying:

```text
admindashboard/app/payment/page.tsx
```

run the project's established validation commands.

At minimum:

* TypeScript
* lint if configured
* production Next.js build

Pay particular attention to:

* client/server boundaries
* `payment_initiate_props`
* query parameters
* missing transaction context
* TypeScript request types
* JSON field names
* hydration issues

Do not introduce build workarounds.

---

# 25. TRANSACTIONS BUILD/TEST

Use the commands specified by:

```text
transactions/Makefile
transactions/.clinerules.md
```

Run the appropriate:

* Go formatting
* Go tests
* Go build
* protobuf generation if required
* sqlc generation if required
* migration validation
* lint/static analysis if configured

Do not invent a new validation process.

---

# 26. IMPORTANT — CHECK THE ACTUAL REQUEST LOG

The current AWS log was:

```text
{"level":"info","time":"2026-09-02T09:53:06Z","caller":"github.com/I-Frostbyte/rvpay-go/transactions/deposits/service.go:52","message":"Initate Deposit request body: amount:{amount:\"100.00\" currency:\"XAF\"} payment_type:PAYMENT_TYPE_MMO payer_phone_number:\"+237654131027\" provider:PROVIDER_MTN_MOMO"}
```

This proves the request currently reaches the Transactions service but:

```text
client_id
customer_id
merchant_id
```

are absent.

The implementation must ensure that after this task the equivalent log contains the expected external identifiers, conceptually:

```text
client_name:"highlevel-..."
customer_id:"..."
merchant_id:"..."
amount:{...}
payment_type:PAYMENT_TYPE_MMO
payer_phone_number:"+237..."
provider:PROVIDER_MTN_MOMO
```

Do not merely make the frontend believe it sent the values.

Verify the actual request entering `InitiateDeposit`.

---

# 27. LOGGING

Preserve useful diagnostics.

It is acceptable to update the InitiateDeposit request logging so that the new identifier fields are visible.

However:

* do not log secrets
* do not log API keys
* do not log access tokens
* do not log client secrets
* do not expose sensitive authentication material

The GHL transaction/contact/location identifiers are expected correlation identifiers for this flow and may be logged according to existing project conventions.

Follow `.clinerules.md`.

---

# 28. SCOPE CONTROL

Do NOT:

* redesign the payment UI
* redesign the HighLevel handshake
* remove the working `payment_initiate_props` implementation
* modify the PawaPay SDK
* create a new microservice
* create a new payment endpoint
* change Docker architecture
* change AWS infrastructure
* change authentication architecture
* generate fake UUIDs
* generate fake transaction IDs
* use `orderId` as transactionId
* use `buyNowProductId` as transactionId
* globally remove UUIDs from RVPay
* rewrite unrelated Transactions services
* modify historical database migrations
* delete production data
* bypass database validation
* bypass tenant validation without understanding/replacing it correctly

Only make changes required to implement the new identifier contract end-to-end.

---

# 29. REQUIRED END-TO-END DATA CONTRACT

When finished, the following must be true.

## HighLevel

```text
payment_initiate_props.locationId
payment_initiate_props.contact.id
payment_initiate_props.transactionId
```

↓

## Admin Dashboard

```text
client_name = existing RVPay HighLevel client-name convention using locationId

customer_id = payment_initiate_props.contact.id

merchant_id = payment_initiate_props.transactionId
```

plus:

```text
amount
currency
payment_type
payer_phone_number
provider
```

↓

## Public API

```text
POST /v1/public/deposits
```

↓

## Transactions service

NO:

```go
uuid.Parse(client_name)
uuid.Parse(customer_id)
uuid.Parse(transaction_id)
```

The three identifiers must remain strings.

↓

## Persistence

The three identifiers must be stored using the new string-based schema:

```text
client_name
customer_id nullable
merchant_id nullable
```

↓

## PawaPay

Only the actual payment/provider fields are passed into PawaPay.

---

# 30. FINAL REPOSITORY SEARCH

Before declaring completion, perform repository searches for:

```text
uuid.Parse(req.GetClientId())
uuid.Parse(req.GetCustomerId())
uuid.Parse(req.GetMerchantId())

GetClientId()
GetCustomerId()
GetMerchantId()

client_id UUID
customer_id UUID
merchant_id UUID
```

and equivalent SQL/schema declarations.

Confirm that no stale UUID assumptions remain in the affected deposit path.

Also search:

```text
client_name
```

to ensure the new identifier is consistently implemented.

---

# 31. REQUIRED DOCUMENTATION UPDATES

At the END of the task, append ONLY the changes made during this task.

Do NOT rewrite existing history.

Do NOT replace existing content.

Do NOT summarize unrelated previous work.

Append a dated/task-specific section to:

```text
admindashboard/.clinecheck.md
admindashboard/.project-checkpoint.md
transactions/.clinecheck.md
transactions/.service-checkpoint.md
```

IMPORTANT:

The fourth path is:

```text
transactions/.service-checkpoint.md
```

NOT:

```text
trasnactions/.service-checkpoint.md
```

Only append information relevant to THIS task.

The appended information should identify:

* identifier mapping change
* HighLevel source fields
* Admin Dashboard request mapping
* Transactions contract change
* migration created
* persistence changes
* UUID validation removal
* tests/builds performed
* remaining limitations

Do not modify `.clineignore` or `.clinerules` files.

---

# 32. REQUIRED FINAL REPORT

When finished, provide a precise report containing:

## 1. Files changed

List every changed file.

Separate:

```text
Admin Dashboard
Transactions
Database migration
Generated files
Documentation/checkpoints
```

Do not omit generated files.

---

## 2. HighLevel mapping

Explicitly state:

```text
locationId → client_name
contact.id → customer_id
transactionId → merchant_id
```

and explain exactly how `client_name` is constructed.

---

## 3. API contract

State the final `CreateDepositRequest` shape.

Show the relevant fields.

---

## 4. Backend validation

Explain exactly which UUID validations were removed and what validation now replaces them.

---

## 5. Database

Identify:

* migration filename
* old column types
* new column types
* nullability changes
* foreign key/index changes
* whether existing data is preserved

---

## 6. Persistence

Explain how the values travel:

```text
presentation
→ service
→ repository
→ SQL
→ database
```

---

## 7. Payment flow

Confirm:

```text
GHL
→ payment page
→ POST /v1/public/deposits
→ Transactions service
→ persistence
→ PawaPay
```

still works conceptually.

---

## 8. Status flow

Confirm the existing:

```text
transactionId
→ /v1/public/payments/verify
```

flow remains intact.

---

## 9. Tests/builds

Report the actual commands run and their results.

Do NOT claim a command passed if it could not be executed.

Distinguish:

```text
verified by source inspection
verified by unit/integration test
verified by local build
verified against live AWS
verified against live HighLevel
```

---

## 10. Remaining limitations

Clearly state anything that still requires a live deployment or live HighLevel payment test.

In particular, do not claim that a successful local build proves:

```text
HighLevel iframe
→ payment_initiate_props
→ Admin Dashboard
→ AWS API
```

works end-to-end.

---

# ACCEPTANCE CRITERIA

Do not report completion unless ALL applicable criteria are satisfied.

### A. HighLevel context

The working `payment_initiate_props` implementation remains intact.

### B. Client mapping

```text
locationId → client_name
```

using the repository's actual HighLevel client-name convention.

### C. Customer mapping

```text
contact.id → customer_id
```

without UUID parsing.

### D. Merchant mapping

```text
transactionId → merchant_id
```

without UUID parsing.

### E. API

The existing:

```text
POST /v1/public/deposits
```

receives all three identifiers.

### F. Transactions

No UUID validation is performed on the three external identifier fields.

### G. Database

A NEW migration changes the affected identifier columns from UUID-specific storage to appropriate string storage.

### H. Nullability

`customer_id` is nullable.

`merchant_id` is nullable.

### I. Persistence

The exact HighLevel identifier values survive through repository/persistence into the database.

### J. Payment fields

These remain correct:

```text
amount
currency
payment_type
payer_phone_number
provider
```

### K. PawaPay

Existing PawaPay initiation behavior remains intact.

### L. Status

Existing GHL transaction verification/polling remains intact.

### M. No fake identifiers

No generated/random UUIDs are used as replacements for the GHL identifiers.

### N. Migration safety

No historical migration was modified.

### O. Tests/build

Relevant tests and builds pass, or failures are explicitly documented.

### P. Documentation

All four requested checkpoint files are APPENDED to, not rewritten.

### Q. Scope

No unrelated architecture or UI changes were introduced.

Proceed now. Inspect the CURRENT repository first, then implement the complete change end-to-end.

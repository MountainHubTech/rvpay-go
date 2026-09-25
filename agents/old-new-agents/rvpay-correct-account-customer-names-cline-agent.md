# Cline Agent — Correct HighLevel Account and Customer Names

## Mission

Fix RVPay so the Admin Dashboard displays real human-readable HighLevel sub-account names and customer names instead of internal identifiers.

Current behavior:
- Account name is `highlevel-<locationId>`.
- Customer name is the internal RVPay `customerId`.

Desired behavior:
- `clients.name` (or the existing account-name field) contains the authoritative HighLevel location/sub-account name.
- Customer name contains the authoritative customer/contact name supplied in the HighLevel payment context.
- Internal IDs remain intact for correlation and database relationships. Do not discard or replace them.

The developer will add the HighLevel OAuth scope:

```text
locations.readonly
```

This permits an authenticated GET of the HighLevel location body so the actual sub-account name can be retrieved.

For customers, use the name already present in the HighLevel transaction/payment initiation request. If the existing protobuf does not carry the name, add the smallest backward-compatible field and propagate it from the Admin Dashboard.

This task includes a safe, repeatable mechanism to correct existing database records as well as future records.

---

## 1. Mandatory Reading

Before changing anything, read ALL of these files:

```text
.clinecheck
.clinerules
.clineignore
README.md

admindashboard/.project-checkpoint.md
admindashboard/.clinerules.md
admindashboard/.clineignore.md
admindashboard/.clinecheck.md
admindashboard/README.md

clients/.service-checkpoint.md
clients/.clinerules.md
clients/.clineignore.md
clients/.clinecheck.md
clients/README.md

transactions/.service-checkpoint.md
transactions/.clinerules.md
transactions/.clineignore.md
transactions/.clinecheck.md
transactions/README.md
```

Do not skip these files. Do not overwrite them.

---

## 2. Context-Minimization Rule

After reading the mandatory files:

1. Build a short internal map of the relevant data flow.
2. Use targeted searches rather than dumping large source files into context.
3. Read only relevant functions/types around search results.
4. Do not inspect unrelated packages unless directly involved.
5. Do not repeatedly reread files already understood.
6. For generated code, inspect only enough to understand the contract; regenerate it using the repository's established process rather than hand-editing it.
7. Keep the final report concise but technically complete.

Search first for:

```text
clientName
client_name
customerName
customer_name
customerId
customer_id
locationId
location_id
highlevel-
external_account_id
CreateDepositRequest
payment_initiate_props
InitiateDeposit
customer
client
Location
locations
name
firstName
lastName
```

---

## 3. Inspect the Existing Data Model

Determine exactly where these values currently live.

### Client/account

Find:
- client table/model;
- client repository;
- client creation/update path;
- HighLevel INSTALL/reconciliation path;
- `external_account_id`;
- current name field;
- every place `highlevel-<locationId>` is created or returned.

Determine whether the current name is persisted, derived, or both.

### Customer

Find:
- customer table/model;
- customer repository;
- customer creation/update path;
- customer lookup during payment initiation;
- current name field;
- every place `customerId` is used as a displayed/customer name.

Do not assume the schema.

---

## 4. HighLevel Location Name

Inspect the existing HighLevel provider/authentication implementation.

Determine:
1. How stored access tokens are retrieved.
2. How the authoritative location ID is obtained.
3. Whether an authenticated GET helper already exists.
4. Whether a location API method/model already exists.
5. How INSTALL and reconciliation currently create/update clients.
6. Where the location name should be fetched with the smallest architectural change.

Use the existing HighLevel authentication/provider patterns. Do not create a second auth system.

Use the documented HighLevel location endpoint appropriate to `locations.readonly`.

The HighLevel location ID remains the authoritative identifier. The location's human-readable name becomes the account/client name.

Never derive the final display name from the location ID.

---

## 5. Existing Client Backfill

This is mandatory.

Determine how many affected records exist and how they are identified safely.

Build an idempotent application-level backfill/reconciliation mechanism if external HighLevel API calls are required. A SQL migration must not call HighLevel.

For each affected client:

```text
existing client
  ↓
authoritative external_account_id/locationId
  ↓
stored HighLevel token
  ↓
refresh if necessary using existing auth logic
  ↓
GET HighLevel location
  ↓
extract authoritative location name
  ↓
update existing client name
```

Preserve:
- client ID;
- `external_account_id`;
- customer relationships;
- OAuth/install identity.

Do not embed live names, tokens, or URLs in migrations/source.

The operation must be safe to rerun.

If a record cannot be corrected, preserve its existing value and report the reason. Never replace a valid name with an ID or error text.

Handle at least:
- missing token;
- expired/refreshable token;
- refresh failure;
- 401;
- 403;
- 404;
- 429;
- 5xx;
- timeout/network error;
- empty/malformed location name.

Do not fabricate names.

Clearly distinguish in the final report between:
- backfill implemented;
- backfill tested;
- backfill executed;
- backfill verified.

Do not claim production execution unless it actually happened.

---

## 6. Future INSTALL/Reconciliation

Update the existing HighLevel INSTALL/reconciliation path so a new sub-account follows:

```text
OAuth/token exchange
  ↓
location ID established
  ↓
authenticated GET location
  ↓
location name
  ↓
create/update client
```

Requirements:
- no duplicate clients;
- existing install/reinstall idempotency preserved;
- `external_account_id` remains the HighLevel location ID;
- actual location name is persisted;
- API failure must not destroy a valid existing name;
- do not fall back to `highlevel-<locationId>` as the final name.

If existing reauthorization/reconciliation can safely refresh the location name, reuse that path rather than adding a parallel lifecycle.

---

## 7. Customer Name Source

Inspect the actual Admin Dashboard `payment_initiate_props` structure and payment initiation flow.

Use the customer/contact name already supplied by HighLevel.

Do not derive a name from:
- customer ID;
- UUID;
- phone;
- email;
- location ID.

If the payment context contains first/last name fields, preserve that established representation.

The customer ID must remain available for internal correlation.

Conceptually the request should carry both:

```text
customerId   = identifier
customerName = human-readable name
```

If an authoritative name is absent, do not substitute the customer ID.

---

## 8. Protobuf / Transactions Path

Inspect the source protobuf used by `CreateDepositRequest` and trace:

```text
Admin Dashboard
  ↓
HTTP/gRPC gateway
  ↓
CreateDepositRequest
  ↓
Transactions service
  ↓
customer lookup/create/update
  ↓
database
```

If the protobuf already has a suitable customer-name field, use it.

If it does not:
1. Add the smallest appropriate backward-compatible field.
2. Follow existing protobuf naming/numbering conventions.
3. Never reuse a field number.
4. Regenerate generated protobuf/gateway code with the established repository process.
5. Never hand-edit generated files.
6. Do not change unrelated messages.

Then propagate the field through the service so:
- new customers get the actual name;
- existing customers can have an ID-derived placeholder corrected;
- customer ID remains unchanged;
- no duplicate customer is created because a name changes.

---

## 9. Customer Update Semantics

Implement explicit safe rules:

- New customer + authoritative name → store name.
- Existing customer with ID-derived placeholder + authoritative name → replace placeholder.
- Existing customer with real name + empty incoming name → do not erase it.
- Existing customer with real name + new authoritative name → follow existing customer-update semantics and document the decision.
- Never identify customers by name.
- Never replace the customer ID with the name.

---

## 10. Existing Customer Backfill

First determine the exact corruption pattern rather than assuming every UUID-looking name is wrong.

For existing customers, inspect historical persisted transaction/payment data for authoritative:
- customer name;
- first/last name;
- contact ID;
- other existing authoritative customer metadata.

Use only persisted authoritative data.

If the name can be recovered, implement a safe idempotent backfill.

If it cannot be recovered, do not invent it. Report exactly which data is missing and why those records cannot be automatically corrected.

Preserve:
- customer ID;
- contact/external identifiers;
- phone;
- email;
- client relationship.

Only correct the human-readable name.

---

## 11. Admin Dashboard

Update the payment initiation request so it passes the actual customer name separately from the customer ID.

Do not remove the customer ID.

Do not fabricate a customer name.

After the backend changes, verify dashboard data uses:
- account/client name for display;
- customer name for display;
- IDs only where identifiers/correlation are required.

Remove only incorrect ID-to-display-name derivation. Do not redesign the dashboard.

---

## 12. OAuth Scope

The new scope is:

```text
locations.readonly
```

Update any source-controlled scope definition only if the repository contains one.

Do not add unrelated scopes.

Inspect whether existing installations need reinstall/reauthorization to receive the newly requested scope. Document that operational requirement if applicable.

Do not invent an OAuth migration mechanism.

---

## 13. Database / Backfill Rules

Do not add duplicate name columns if existing fields are sufficient.

Separate:
- schema changes;
- data correction/backfill.

If external HighLevel API calls are required, use application code with existing token handling.

The backfill must:
- be idempotent;
- preserve identifiers;
- continue past independent failures where architecture allows;
- report failures;
- never fabricate data.

---

## 14. Security

Never log or store:
- access tokens;
- refresh tokens;
- authorization headers;
- secrets;
- production webhook URLs.

Avoid unnecessary customer PII in logs.

Safe logs may include:
- client ID;
- external account/location ID;
- customer ID;
- operation;
- HTTP status;
- success/failure.

Use existing observability conventions.

---

## 15. Tests

Add/update focused tests for:

### Location
- successful authenticated location GET;
- location ID;
- location-name extraction;
- empty name;
- 401/403/404/429/5xx;
- network/timeout where existing conventions support it.

### Client
- new client gets actual location name;
- external account ID remains unchanged;
- existing client updated, not duplicated;
- valid name not replaced by fallback/error;
- reinstall/reconciliation remains idempotent.

### Customer
- new customer gets actual name;
- ID remains unchanged;
- ID-derived placeholder is corrected;
- real name is not overwritten with empty input;
- duplicate customer is not created.

### Protobuf/API
If changed:
- regenerated code;
- gateway/service path;
- backward compatibility for requests without the optional name.

### Dashboard
Verify:
- actual name is read from payment context;
- customer ID still passes where required;
- customer name is passed separately;
- payment initiation still works.

### Backfill
Test:
- already-correct client;
- placeholder client;
- token failure;
- API failure;
- empty location name;
- already-correct customer;
- ID-derived customer name;
- recoverable historical customer name;
- unrecoverable customer name;
- repeated execution.

No live production credentials in tests.

---

## 16. Documentation — APPEND ONLY

When complete, append — never overwrite — to each affected service's:

```text
.clinecheck
README.md

admindashboard/.clinecheck.md
admindashboard/README.md
admindashboard/.project-checkpoint.md

clients/.clinecheck.md
clients/README.md
clients/.service-checkpoint.md

transactions/.clinecheck.md
transactions/README.md
transactions/.service-checkpoint.md
```

Only update a service's files if that service was actually affected.

Each appended entry must include:
- date;
- agent/task name;
- status;
- exact files changed;
- account-name source;
- customer-name source;
- `locations.readonly` dependency;
- client backfill mechanism;
- customer backfill mechanism;
- protobuf/API changes;
- dashboard changes;
- database changes;
- manual backfill execution instructions;
- OAuth reauthorization implications;
- tests/results;
- build/lint/vet results;
- known limitations;
- records that could not be corrected automatically;
- next task.

Never delete or rewrite earlier entries.

---

## 17. Verification

Run the relevant established checks.

For Clients:

```bash
go test ./clients/... -count=1
go vet ./clients/...
```

For Transactions:

```bash
go test ./transactions/... -count=1
go vet ./transactions/...
```

For the dashboard, inspect `admindashboard/package.json` and run the applicable existing scripts such as:

```bash
npm test
npm run lint
npm run build
```

Only run commands that actually exist in the repository.

If protobuf regeneration is required, use the established generation command.

Do not fix unrelated failures.

---

## 18. Final Report

Return:

### 1. Implementation Summary
What changed and why.

### 2. Account Name Flow
Show:

```text
HighLevel location
  ↓
location API
  ↓
location name
  ↓
clients database
  ↓
dashboard
```

### 3. Customer Name Flow
Show:

```text
HighLevel payment context
  ↓
dashboard
  ↓
protobuf/request
  ↓
transactions
  ↓
customer database
  ↓
dashboard
```

### 4. Existing Data Backfill
Explain identification, authoritative source, idempotency, execution status, and any failures.

### 5. Files Changed
Separate root, dashboard, clients, transactions, protobuf/generated code, and migrations/backfill.

### 6. OAuth/API Changes
Explain `locations.readonly`, location GET, protobuf additions, and compatibility.

### 7. Tests and Verification
Exact commands/results.

### 8. Known Limitations
Only real limitations.

### 9. Manual Operator Steps
Especially:
- add `locations.readonly`;
- reinstall/reauthorize if required;
- execute backfill if not executed;
- deploy/restart if required.

### 10. Next Task
Smallest remaining task, if any.

---

## 19. Completion Criteria

Do not declare completion until:

1. New HighLevel sub-accounts can receive their actual location name.
2. Existing affected clients have a safe repeatable correction mechanism.
3. Client IDs are preserved.
4. HighLevel location IDs are preserved.
5. New customers receive actual names when supplied by payment context.
6. Customer IDs are preserved.
7. Customer IDs are no longer used as display names when an authoritative name exists.
8. Existing recoverable customer names have a safe backfill path.
9. Unrecoverable customer names are reported, not fabricated.
10. Dashboard passes customer name separately from customer ID.
11. Protobuf carries customer name if needed.
12. Generated code is regenerated properly.
13. Existing payment behavior remains intact.
14. Existing OAuth/install/reconciliation behavior remains intact apart from necessary name enrichment.
15. `locations.readonly` is the only new HighLevel scope required.
16. No credentials/secrets are exposed.
17. Affected checkpoints/rules/README files are appended to.
18. Relevant tests and validation pass.
19. `highlevel-<locationId>` is not used as the final account display name.
20. Internal customer ID is not used as the final customer display name when an authoritative name exists.

Do not fabricate production row counts or claim that a production backfill was executed unless it actually was.

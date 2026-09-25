# Cline Task — Verify HighLevel Tokens, Diagnose Sub-Accounts 500, and Fix Dashboard Name Mapping

## Task Type

**Targeted diagnosis + implementation + verification**

Repository:

`github.com/MountainHubTech/rvpay-go`

The previous account/customer-name implementation has already been deployed, but the following problems remain:

1. Admin Dashboard Sub-Accounts still returns HTTP 500.
2. Transactions still display `highlevel-<locationId>` instead of the actual HighLevel sub-account name.
3. Transactions still display the customer ID instead of the customer name.
4. The developer added the required OAuth scope(s) and re-installed the RVPay app in both sub-accounts(locationId):
   - `lamyP5Es8Q2nQrb90fuv`
   - `rVBsqXAKXlYLz0C96kA8`
5. The developer specifically wants proof that the application is actually using the newly issued OAuth access tokens.
6. The Admin Dashboard must be verified against **actual runtime backend responses**, not merely source/types.

## Runtime prerequisite

Before running this task, Docker Desktop will already be running and the local RVPay containers/services will be started.

**Runtime verification is mandatory. Do not skip it because source inspection appears sufficient.**

---

# 1. Required Control Files

Read only the relevant project-control files first:

### Root
- `.clinecheck`
- `.clinerules`
- `.clineignore`
- `README.md`

### Admin Dashboard
- `admindashboard/.project-checkpoint.md`
- `admindashboard/.clinerules.md`
- `admindashboard/.clineignore.md`
- `admindashboard/.clinecheck.md`
- `admindashboard/README.md`

### Clients
- `clients/.service-checkpoint.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/README.md`

### Transactions
- `transactions/.service-checkpoint.md`
- `transactions/.clinerules.md`
- `transactions/.clineignore.md`
- `transactions/.clinecheck.md`
- `transactions/README.md`

Also inspect the existing account/customer-name implementation and its documentation/checkpoint entries.

Do not read the whole repository indiscriminately.

---

# 2. Git Baseline

Before changing anything:

```bash
git status --short
git diff --stat
git diff --name-only
git log -10 --oneline
```

Record the baseline.

Do not revert unrelated existing work.

Do not create a commit unless repository rules explicitly require it.

---

# 3. Runtime Verification — Mandatory

Because Docker will already be running, verify the actual environment.

Determine the repository's existing local startup mechanism from its documentation.

Verify:

```text
Docker
  ↓
database
  ↓
Clients
  ↓
Transactions
  ↓
Admin Dashboard
```

Capture:

- relevant container/service names;
- running/healthy state;
- relevant ports;
- startup errors;
- migration output/status.

Do not expose environment secrets.

---

# 4. Reproduce the Sub-Accounts 500

Reproduce the actual Dashboard request:

```http
GET /v1/public/clients/sub-accounts?page=1&pageSize=20
```

Capture:

- exact HTTP status;
- response body;
- relevant Clients logs;
- underlying repository/SQL error;
- whether `clients.display_name` exists in the active database;
- whether migration `000006` is applied.

Do **not** merely repeat the previous hypothesis that this is a migration mismatch. Prove the actual runtime cause.

If the endpoint now succeeds, continue anyway and inspect the actual payload and Dashboard rendering.

---

# 5. Verify Database Schema and Name Data

Using read-only inspection where possible, verify:

```sql
clients.display_name
```

exists.

For both locations:

- `lamyP5Es8Q2nQrb90fuv`
- `rVBsqXAKXlYLz0C96kA8`

inspect relevant non-secret records and metadata:

- location ID;
- integration/client record;
- `client_name`;
- `display_name`;
- provider/integration identifier;
- created/updated timestamps;
- token metadata if available;
- scope metadata if available.

Never print access tokens, refresh tokens, client secrets, authorization codes, or encrypted credential blobs.

Report whether each `display_name` is:

- correct;
- stale;
- NULL;
- empty;
- still the `highlevel-<locationId>` fallback.

---

# 6. Prove Which OAuth Token Is Being Used

This is a critical requirement.

The developer has already:

- added the required OAuth scope;
- re-installed the app in both locations.

Do **not** merely inspect the source and say the scope is requested.

Trace the actual credential lifecycle:

```text
HighLevel INSTALL
    ↓
OAuth/token exchange
    ↓
stored credential
    ↓
credential selected for location
    ↓
location-name request
    ↓
HighLevel response
    ↓
display_name
```

For each location, establish as far as the repository/runtime permits:

1. which integration/client record corresponds to the location;
2. whether a new access token was stored during the latest installation;
3. token creation/update timestamp if available;
4. token expiry if available;
5. whether the stored credential is the one selected at runtime;
6. whether `locations.readonly` is associated with that credential;
7. whether that credential is actually used for the HighLevel location request.

Use safe evidence such as:

- non-secret credential record ID;
- timestamps;
- expiry;
- scopes;
- short non-reversible token fingerprint if necessary.

If the application has an existing safe fingerprint/redaction mechanism, use it.

Do not add credential-exposing logging merely to perform this diagnostic.

Do not re-install the app yourself.

Do not change OAuth URLs/grant types/scopes unless runtime/source evidence proves the current implementation is wrong.

---

# 7. Token Verification Table

For each location, produce:

```text
Location:
Credential record:
Token updated:
Token expires:
locations.readonly:
Credential selected by runtime:
Evidence:
```

Never include raw token material.

If HighLevel does not expose scopes in a way that permits direct verification, say so explicitly and provide the strongest available evidence. Do not claim scope verification merely because the source contains the scope string.

---

# 8. Verify HighLevel Location Name Enrichment End-to-End

Verify the complete path for both locations:

```text
active OAuth credential
        ↓
GET /locations/{locationId}
        ↓
HighLevel location name
        ↓
clients.display_name
        ↓
SubAccountRow.name
        ↓
HTTP JSON response
        ↓
Dashboard
        ↓
rendered name
```

Verify:

- correct location ID;
- actual request URL/path;
- expected v3 headers;
- actual HighLevel response status;
- actual returned location name;
- persisted `clients.display_name`;
- `SubAccountRow.name`;
- actual JSON field/value;
- Dashboard rendered value.

Do not assume the path is working merely because the code exists.

---

# 9. Verify the Sub-Account Backend Contract

Trace:

```text
protobuf
  ↓
generated Go
  ↓
Clients converter/service
  ↓
HTTP/JSON response
  ↓
admindashboard/lib/api.ts
  ↓
sub-accounts page
  ↓
table renderer
```

Specifically verify:

- `SubAccountRow.name`
- `SubAccountRow.client_name`
- `Client.display_name`

Verify the actual JSON naming convention and response payload.

If the backend returns something like:

```json
{
  "name": "Actual HighLevel Location Name",
  "client_name": "highlevel-rVBsqXAKXlYLz0C96kA8"
}
```

the Dashboard must display `name`, not `client_name`.

---

# 10. Verify the Transactions Backend

The intended contract is:

```text
TransactionListRow.customer
    = customer ID / external identifier

TransactionListRow.customer_name
    = human-readable customer name
```

Verify the actual running endpoint response.

Trace:

```text
transactions/overview/service.go
        ↓
TransactionListRow.CustomerName
        ↓
generated protobuf
        ↓
HTTP JSON
        ↓
Dashboard API type
        ↓
transactions page
        ↓
transactions-view.tsx
```

Do not assume `customer_name` works because it exists in `.proto`.

If it is absent or empty, identify exactly why.

---

# 11. Verify Customer Name Resolution

Inspect the existing customer SQL/repository/service implementation.

Determine:

- authoritative source of customer names;
- whether lookup by customer ID already exists;
- whether the repository interface exposes it;
- whether `deposit.CustomerID` matches the customer table identifier;
- whether customer names can be resolved dynamically without a new transaction schema column.

Prefer the smallest implementation consistent with existing patterns.

Do not create a transaction `customer_name` database column unless the existing architecture clearly requires it.

Do not invent a historical backfill unless evidence shows it is required.

Preserve:

```text
TransactionListRow.customer = customer ID
```

and use:

```text
TransactionListRow.customer_name
```

as the human-readable value.

---

# 12. Verify the Admin Dashboard Against ACTUAL Responses

This is mandatory.

Do not conclude the Dashboard is correct from TypeScript source alone.

With the local services running:

1. request the Sub-Accounts endpoint;
2. capture actual JSON;
3. request the Transactions endpoint used by the page;
4. capture actual JSON;
5. compare response fields with Dashboard types;
6. verify the actual pages render the correct values.

## Sub-Accounts page

Verify:

```text
backend SubAccountRow.name
        ↓
Dashboard API response type
        ↓
page model
        ↓
table
        ↓
visible account name
```

It must not use `client_name` as the human-readable account name.

## Transactions page

Verify:

```text
backend customer_name
        ↓
Dashboard API type
        ↓
transactions view
        ↓
visible customer name
```

Use this fallback:

```text
customer_name if non-empty
otherwise customer
```

The customer ID must remain available for correlation.

If the actual response uses a different JSON property name because of gateway/protobuf configuration, make the Dashboard match the actual contract rather than guessing.

---

# 13. Fix Confirmed Defects Only

If runtime verification identifies concrete defects, fix them.

Allowed scope:

### Clients
Only changes required for:

- the actual Sub-Accounts 500;
- location-name enrichment;
- correct credential selection/storage;
- correct `SubAccountRow.name` mapping.

### Transactions
Only changes required for:

- populating `TransactionListRow.customer_name`;
- preserving `TransactionListRow.customer`;
- resolving authoritative customer names.

### Admin Dashboard
Only changes required for:

- correctly interpreting the actual backend response;
- correctly mapping JSON fields;
- rendering account names;
- rendering customer names;
- safe fallbacks.

Do not touch:

- native GHL order synchronization;
- inbound webhook workflows;
- unrelated OAuth flows;
- unrelated pages;
- unrelated database design.

Do not create a second OAuth implementation.

---

# 14. Generated Code

If `.proto` changes are necessary:

- follow the repository's established generation workflow;
- regenerate rather than manually editing generated files;
- verify generated output.

If no protobuf change is needed, do not regenerate unnecessarily.

---

# 15. Tests

Run the narrowest relevant tests after implementation.

At minimum, where applicable:

```bash
go test ./clients/...
go test ./transactions/...
```

Run the existing Admin Dashboard checks documented by its control files.

Also run targeted tests for:

- OAuth token exchange/storage/selection;
- location-name enrichment;
- transaction customer-name mapping;
- Dashboard mapping if tests exist.

---

# 16. Runtime Verification After Fixes

After source changes:

1. rebuild/restart affected containers/services;
2. verify migrations;
3. request Sub-Accounts;
4. request Transactions;
5. inspect actual JSON;
6. load the Dashboard pages;
7. verify rendered values.

For both:

```text
lamyP5Es8Q2nQrb90fuv
rVBsqXAKXlYLz0C96kA8
```

Do not declare success unless runtime behavior supports it.

---

# 17. Credential Safety

Never include in the report:

- access tokens;
- refresh tokens;
- client secrets;
- authorization codes;
- Authorization headers;
- encrypted credential blobs.

Safe evidence may include:

- non-secret record IDs;
- timestamps;
- expiry;
- scope names;
- short one-way fingerprints;
- yes/no runtime selection evidence.

---

# 18. Documentation

If source changes are made:

- append to the relevant checkpoint/documentation files;
- do not overwrite existing documentation;
- distinguish:
  - implemented;
  - executed;
  - verified.

Do not create a broad new project document.

Do not modify `.clineignore` or `.clinerules` unless repository rules require it.

---

# 19. Required Final Report

Use exactly this structure:

## 1. Executive Finding

Separate the actual root causes for:

- Sub-Accounts 500;
- sub-account display name;
- customer display name;
- OAuth token/scope;
- Dashboard contract/rendering.

## 2. Runtime Status

```text
Docker:
Database:
Clients:
Transactions:
Dashboard:
```

## 3. Sub-Accounts 500

```text
Endpoint:
HTTP status before fix:
Actual error:
Root cause:
Fix:
HTTP status after fix:
```

## 4. HighLevel OAuth Token Verification

| Location | New-token evidence | `locations.readonly` | Runtime selected new token | Location API result |
|---|---|---|---|---|
| `lamyP5Es8Q2nQrb90fuv` | ... | ... | ... | ... |
| `rVBsqXAKXlYLz0C96kA8` | ... | ... | ... | ... |

Never include credentials.

## 5. Account Name Flow

```text
HighLevel location name
→ clients.display_name
→ SubAccountRow.name
→ JSON
→ Dashboard
→ rendered account name
```

State the observed result for both locations.

## 6. Customer Name Flow

```text
customers.name
→ TransactionListRow.customer_name
→ JSON
→ Dashboard API type
→ transactions page
→ rendered customer name
```

State exactly where it was broken and what was changed.

## 7. Dashboard Contract Verification

| Backend | JSON | Dashboard | Rendered |
|---|---|---|---|
| `SubAccountRow.name` | ... | ... | ... |
| `SubAccountRow.client_name` | ... | ... | ... |
| `TransactionListRow.customer` | ... | ... | ... |
| `TransactionListRow.customer_name` | ... | ... | ... |

Explicitly state whether the Dashboard was verified against actual runtime responses.

## 8. Files Changed

Group actual changes under:

- Clients
- Transactions
- Admin Dashboard
- Generated code
- Tests
- Documentation

## 9. Verification

List exact commands and runtime checks and their results.

## 10. Remaining Issues

Only report genuine remaining issues. Do not speculate.

---

# 20. Final Git Safety Check

Before finishing:

```bash
git status --short
git diff --stat
git diff --name-only
```

Confirm every modification is directly related to this task.

Identify pre-existing unrelated changes separately.

---

# 21. Success Criteria

The task is successful only if the evidence answers:

1. Why does `/v1/public/clients/sub-accounts` return 500?
2. Does the active database contain `clients.display_name`?
3. Is the correct/new OAuth credential stored for `lamyP5Es8Q2nQrb90fuv`?
4. Is the correct/new OAuth credential stored for `rVBsqXAKXlYLz0C96kA8`?
5. Is `locations.readonly` actually associated with the credential being used?
6. Can runtime evidence identify which stored credential is selected?
7. Does the HighLevel location endpoint return the actual location name for both locations?
8. Is that name persisted to `clients.display_name`?
9. Does `SubAccountRow.name` contain it?
10. Does the actual HTTP response contain the expected field?
11. Does the Dashboard correctly interpret and render it?
12. Does `TransactionListRow.customer_name` actually get populated?
13. Does the actual Transactions response contain it?
14. Does the Dashboard correctly interpret and render it?
15. Is the customer ID preserved?
16. Were both pages verified against actual running responses?
17. Were source changes limited to confirmed defects?
18. Were no credentials exposed?

The final report must clearly distinguish:

```text
Implemented
Executed
Verified
```

Do not declare success merely because code compiles or source inspection looks correct.

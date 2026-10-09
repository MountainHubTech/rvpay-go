# Cline Task — Diagnose Sub-Accounts HTTP 500 with Temporary Server-Side Logging

## Objective

Diagnose the **actual root cause** of the persistent HTTP 500 returned by the RVPay Dashboard's **Sub-Accounts** request.

This task is **diagnostic-first**.

Do not speculate or make broad architectural changes. Add only narrowly scoped, temporary, secret-safe logging around the existing Sub-Accounts request path, reproduce the 500 locally, identify the exact failing operation/record, then make the smallest confirmed fix if the root cause is unambiguous.

### Critical invariant

A HighLevel integration/client with `display_name = NULL` must **not** cause the entire Sub-Accounts list request to return HTTP 500.

Expected fallback:

```text
display_name present -> display_name
display_name NULL    -> highlevel-<locationId>
```

The user has confirmed that on the **testing server**, existing clients currently have no populated `display_name` values. Do not assume that this is itself the cause of the 500.

---

## 1. Required Context Reading

Before inspecting or changing code, read only:

### Repository root
- `.clinecheck`
- `.clinerules`
- `.clineignore`
- `README.md`

### Dashboard
- `admindashboard/.project-checkpoint.md`
- `admindashboard/.clinerules.md`
- `admindashboard/.clineignore.md`
- `admindashboard/.clinecheck.md`
- `admindashboard/README.md`

### Clients service
- `clients/.service-checkpoint.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/README.md`

Do not add this task to future Cline context.

Do not overwrite existing checkpoint, rules, ignore, or README files. Append documentation only.

---

## 2. Scope

### In scope

Trace:

```text
Dashboard Sub-Accounts page
  -> Dashboard API client
  -> HTTP request
  -> Clients HTTP handler
  -> Clients service
  -> Clients repository/query
  -> row/model mapping
  -> JSON response
```

Determine exactly where and why HTTP 500 is generated.

Verify that `NULL clients.display_name` is safely handled.

### Explicitly out of scope

Do NOT modify/investigate:

- HighLevel OAuth flow
- OAuth token exchange/scopes
- HighLevel webhooks
- Transactions service
- Customer-name logic
- PawaPay
- CloudFormation
- ECS listener configuration
- production ports
- permanent local port changes
- database architecture
- broad refactors
- unrelated Dashboard lint errors
- unrelated API endpoints

The user has a temporary local workaround for Transactions on port `8081`. Do not change this or any production listener configuration.

---

## 3. Establish the Exact Failure

Before changing code:

1. Start/use the existing local Clients service.
2. Start/use the existing Dashboard.
3. Confirm Clients health responds successfully.
4. Confirm Dashboard points at the intended local Clients API.
5. Open/reload the Sub-Accounts page.
6. Capture the actual request returning 500.

Record:

- HTTP method
- request path
- status
- response body
- timestamp
- authentication requirement
- whether the request reaches Clients

Do not guess the endpoint from memory. Trace the actual Dashboard request.

---

## 4. Add Temporary Secret-Safe Logging

Add narrowly scoped temporary structured logging around the existing Sub-Accounts request path.

The logs must answer:

1. Did the request reach Clients?
2. Which handler received it?
3. Which service method ran?
4. Which repository/query ran?
5. Did the DB query fail?
6. How many rows were returned?
7. Which row failed during mapping/enrichment?
8. Was `display_name` NULL?
9. What fallback was calculated?
10. Did response construction/serialization fail?
11. Exactly where was HTTP 500 generated?

### Safe fields

Log only safe operational identifiers such as:

- request path
- method
- existing request/correlation ID
- handler/service/repository method
- integration ID
- external account/location ID
- `client_name`
- whether `display_name` is present
- row count
- row index
- sanitized error
- final HTTP status

### Never log

- access tokens
- refresh tokens
- client secrets
- OAuth authorization codes
- Authorization headers
- cookies/session tokens
- API keys
- passwords
- full request headers
- payment/customer secrets

If an error may contain credentials, sanitize it before logging.

---

## 5. Instrument the Actual Request Path

Find the implementation actually used by the Sub-Accounts endpoint.

Add temporary logging at these boundaries:

### HTTP handler
Log:
- handler entry
- service call
- service error
- records returned
- final response status

### Service
Log:
- method entry
- repository call
- repository error
- rows returned
- mapping/enrichment start
- mapping/enrichment error

### Repository
Log:
- repository method entry
- query execution
- query error
- rows returned

If sqlc/generated code is involved, do not modify generated code merely for logging unless absolutely necessary. Prefer the repository/service boundary.

### Row mapping
For each returned client/integration row, log only safe identifiers and whether `display_name` exists.

Example:

```text
integration_id=...
external_account_id=...
client_name=...
display_name_present=false
```

If mapping fails, log the exact error and safe row identifiers.

### Response
Determine whether response construction or JSON serialization is actually failing. Do not add unnecessary custom serialization logic.

---

## 6. Reproduce the Failure

After instrumentation:

1. Restart Clients so logging is active.
2. Confirm schema/migrations are valid.
3. Open the Dashboard Sub-Accounts page.
4. Reproduce HTTP 500.
5. Capture the complete relevant server-side log sequence.

Trace:

```text
Dashboard request
 -> HTTP handler
 -> service
 -> repository
 -> database query
 -> row mapping
 -> response construction
 -> JSON serialization
 -> HTTP response
```

Identify the exact stage of failure.

---

## 7. Specifically Test NULL display_name

Use data containing at least one:

```text
display_name = NULL
```

Expected:

```text
name = highlevel-<locationId>
```

and HTTP 200.

A NULL `display_name` must not cause:

- SQL failure
- Go scan failure
- mapping failure
- panic
- HTTP 500

If it does, identify the exact failing line and fix only that defect.

Do **not** populate all display names merely to hide the failure.

---

## 8. Verify Multiple Accounts

The endpoint must list all existing HighLevel integrations/clients available to the authenticated admin.

Verify:

- multiple records are returned;
- one missing `display_name` does not abort the list;
- one optional enrichment failure does not prevent unrelated accounts;
- valid core records still produce HTTP 200.

Do not special-case:

- `lamyP5Es8Q2nQrb90fuv`
- `rVBsqXAKXlYLz0C96kA8`

Those are examples, not the complete account set.

---

## 9. Fix Only the Confirmed Root Cause

Only after reproduction and evidence:

- make the smallest code change required;
- do not perform speculative fixes;
- do not redesign the Sub-Accounts architecture;
- do not add startup scripts/bulk name-population jobs as a substitute for fixing the 500.

Acceptable fixes, if actually proven, include:
- NULL handling;
- SQL scan/nullability;
- row mapping;
- fallback logic;
- optional enrichment incorrectly treated as fatal;
- handler error handling;
- query incorrectly requiring `display_name`;
- response construction.

---

## 10. Tests

Add/update focused tests for the confirmed defect.

At minimum where applicable:

### A — display_name present

```text
display_name = "My GHL Account"
```

Expected:

```text
name = "My GHL Account"
```

### B — display_name NULL

Expected fallback:

```text
name = "highlevel-<locationId>"
```

and HTTP 200.

### C — multiple accounts

Example:

```text
Account A -> display_name present
Account B -> display_name NULL
Account C -> display_name present
```

Expected:
- HTTP 200
- all accounts returned
- B uses fallback
- A/C retain friendly names

### D — optional enrichment failure

If optional name enrichment exists, verify one failed enrichment does not abort the entire list.

---

## 11. Dashboard Verification

Do not change Dashboard code unless the backend response is proven correct and Dashboard independently mishandles it.

After backend verification:

1. Call the actual Sub-Accounts API.
2. Inspect the real JSON.
3. Confirm the expected field is present.
4. Confirm Dashboard consumes it.
5. Reload Sub-Accounts.
6. Confirm HTTP 200.
7. Confirm all returned accounts render.

If backend is correct but Dashboard is wrong, identify the exact Dashboard defect before changing it.

---

## 12. Tests/Checks

Run the narrowest relevant checks first, then:

```bash
go test ./clients/...
go vet ./clients/...
git diff --check
```

If Dashboard source changes, run the narrowest relevant Dashboard checks available.

Do not spend time fixing unrelated pre-existing Dashboard lint failures.

---

## 13. Remove Diagnostic Logging

After identifying/fixing the cause:

- remove verbose temporary tracing;
- retain only concise safe operational logging that fits existing conventions;
- ensure no credential/header logging remains.

Search the diff for:

- `token`
- `secret`
- `authorization`
- `password`
- `cookie`
- `clientSecret`
- `refreshToken`

Ensure no sensitive values were introduced into logs.

---

## 14. Documentation

Append the result to relevant existing checkpoint files. Never overwrite.

Document:

- exact root cause;
- affected code path;
- exact fix;
- tests;
- runtime verification;
- whether NULL fallback was verified;
- whether all available accounts were returned;
- remaining uncertainty.

Do not add this task to future Cline context.

---

## 15. Final Report

Use exactly these sections:

### 1. Root Cause

State exactly where HTTP 500 originated.

```text
HTTP 500 originated in:
Dashboard -> Clients handler -> Service -> Repository -> <exact operation>

Exact error:
<sanitized error>
```

If not reproduced, say so explicitly.

### 2. Evidence

Include:
- request path
- HTTP status
- relevant safe log sequence
- affected operation/row
- query result
- whether `display_name` was NULL
- exact failure point

No secrets.

### 3. Fix Implemented

List exact files and concise changes.

### 4. Tests

List exact commands and results.

### 5. Runtime Verification

State:
- Clients health status
- Sub-Accounts endpoint status
- number of accounts returned
- NULL fallback result
- Dashboard rendering result

### 6. Temporary Logging

State what was added and what was removed/retained.

### 7. Documentation Updated

List checkpoint files updated.

### 8. Remaining Issues

Only evidence-supported issues.

---

# Success Criteria

Complete when either:

## Successful diagnosis/fix

- exact origin of HTTP 500 identified;
- smallest confirmed fix implemented;
- NULL `display_name` safely falls back to `highlevel-<locationId>`;
- multiple accounts return successfully;
- relevant tests pass;
- authenticated Sub-Accounts request returns HTTP 200;
- Dashboard renders successfully;
- no secrets appear in logs.

## Diagnosis blocked

If 500 cannot be reproduced because of authentication, environment, missing data, unavailable services, or another concrete blocker:

- do not guess;
- document exact blocker;
- provide evidence collected;
- leave production behavior unchanged unless instrumentation was necessary;
- state the precise next runtime action required.

Do not claim the issue is fixed without actual authenticated runtime verification.

# Cline Task — Fix Confirmed Sub-Accounts NULL `display_name` Scan Failure

## Objective

Fix **only the confirmed NULL-scan defect** causing the Clients Sub-Accounts endpoint to return HTTP 500.

Runtime evidence is conclusive:

```text
GET /v1/public/clients/sub-accounts?page=1&pageSize=20
```

fails in:

```text
ClientRepo.ListSubAccounts
```

with:

```text
can't scan into dest[4] (col: display_name): cannot scan NULL into *string
```

At least one existing database row has:

```text
clients.display_name = NULL
```

while the repository/sqlc result is scanning that column into a non-null Go `string`.

The intended fallback cannot execute because the row fails during scanning first.

### Primary success condition

A Sub-Accounts request containing clients with `display_name = NULL` must return HTTP 200 instead of HTTP 500.

The NULL value must resolve to the established deterministic fallback:

```text
highlevel-<locationId>
```

---

## 1. Required Context Reading

Before changing code, read:

### Repository root
- `.clinecheck`
- `.clinerules`
- `.clineignore`
- `README.md`

### Clients
- `clients/.service-checkpoint.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/README.md`

### Dashboard

Only read these if needed to verify the response contract:

- `admindashboard/.project-checkpoint.md`
- `admindashboard/.clinerules.md`
- `admindashboard/.clineignore.md`
- `admindashboard/.clinecheck.md`
- `admindashboard/README.md`

Do not add this task to future Cline context.

Do not overwrite existing documentation.

---

## 2. Confirm the Existing Evidence Before Editing

The captured runtime evidence is:

```text
request:
GET /v1/public/clients/sub-accounts?page=1&pageSize=20

operation:
ListSubAccounts

repository:
ClientRepo.ListSubAccounts

error:
can't scan into dest[4] (col: display_name): cannot scan NULL into *string

result:
gRPC Internal
HTTP 500
```

The request has:

```text
status=""
```

Therefore this task is **not** about the previously fixed Dashboard/database status-enum mismatch.

Do not reopen or redesign the status mapping.

---

## 3. Scope

### In scope

Only fix nullable `clients.display_name` handling in the Sub-Accounts listing path.

Desired behavior:

```text
display_name = "Friendly Name"
    -> return "Friendly Name"

display_name = NULL
    -> return client_name
    -> client_name is normally highlevel-<locationId>
```

Prefer a fix at the database/query/sqlc boundary if that matches the existing repository design.

For example, if appropriate to the actual query:

```sql
COALESCE(display_name, client_name)
```

Do not blindly apply this example. Inspect the actual query, sqlc definitions, generated types, and project conventions first.

The requirement is the behavior, not a specific implementation.

### Explicitly out of scope

Do NOT modify:

- HighLevel OAuth
- OAuth scopes
- token exchange
- webhooks
- Transactions
- customer-name logic
- PawaPay
- Dashboard API contract
- Dashboard UI
- CloudFormation
- ECS
- load balancers
- ports
- infrastructure
- migrations solely to populate existing display names
- startup scripts
- bulk display-name backfill
- unrelated SQL queries
- unrelated nullable fields
- unrelated status mapping

Do not solve this by populating all currently NULL `display_name` values.

`NULL` is valid data state and the application must handle it correctly.

---

## 4. Inspect the Actual Query First

Locate the query used by:

```text
ClientRepo.ListSubAccounts
```

Inspect:

- SQL source
- sqlc query definition
- generated row type
- repository method
- service mapping
- existing tests

Determine exactly which column corresponds to:

```text
dest[4] = display_name
```

Confirm whether `display_name` is nullable in PostgreSQL.

Do not assume based solely on previous reports.

---

## 5. Choose the Smallest Correct Fix

Choose the smallest change that makes the repository result compatible with its existing non-null Go field.

If the query has the required fallback relationship, the preferred pattern is conceptually:

```sql
COALESCE(display_name, client_name)
```

so the repository never attempts to scan SQL NULL into a Go `string`.

But base the implementation on the actual query/schema/design.

Preserve:

### Friendly name

```text
display_name = "RVPay Test Account"
```

returns:

```text
"RVPay Test Account"
```

### Fallback

```text
display_name = NULL
client_name = "highlevel-abc123"
```

returns:

```text
"highlevel-abc123"
```

Do not change the meaning of non-null display names.

---

## 6. Be Careful With sqlc / Generated Code

If the SQL query changes and sqlc regeneration is required:

1. Change the source SQL/query definition.
2. Run the project's normal generation command.
3. Inspect the generated diff.
4. Ensure only expected generated changes occur.
5. Do not manually edit generated code when regeneration is available.
6. If generation is unavailable or times out, document that fact rather than inventing generated changes.

Do not modify generated files merely to hide the SQL NULL.

---

## 7. Tests

Add focused regression coverage.

### Test A — NULL display_name

Fixture:

```text
display_name = NULL
client_name = "highlevel-location-a"
```

Expected:

```text
name = "highlevel-location-a"
```

The test must prove the repository no longer fails while scanning the row.

### Test B — populated display_name

Fixture:

```text
display_name = "Friendly Account"
client_name = "highlevel-location-b"
```

Expected:

```text
name = "Friendly Account"
```

### Test C — multiple accounts

Use at least:

```text
Account A:
display_name = "Friendly A"

Account B:
display_name = NULL
client_name = "highlevel-location-b"

Account C:
display_name = "Friendly C"
```

Expected:

```text
Friendly A
highlevel-location-b
Friendly C
```

All three must remain in the response.

### Test D — HTTP/gateway regression

If the existing gateway test framework makes this practical, add/update a focused regression proving the Sub-Accounts endpoint returns HTTP 200 when at least one underlying row has `display_name = NULL`.

Do not create a large integration-test framework for this task.

---

## 8. Runtime Verification

After the code fix:

1. Start Clients in the normal local configuration.
2. Ensure configured PostgreSQL is running.
3. Start Dashboard if needed.
4. Authenticate normally.
5. Open the Sub-Accounts page.

Verify:

```text
GET /v1/public/clients/sub-accounts?page=1&pageSize=20
```

returns:

```text
HTTP 200
```

Inspect the response.

At minimum verify:

- accounts with friendly names retain friendly names;
- accounts with NULL `display_name` appear;
- NULL accounts use `highlevel-<locationId>`;
- no account disappears because its display name is NULL.

If authenticated runtime verification cannot be performed, do not claim it was verified. Report the exact blocker.

---

## 9. Do Not Add New Diagnostic Logging

The previous diagnostic task already established the exact failure:

```text
cannot scan NULL into *string
```

Do not add another round of verbose tracing.

Existing safe logging may remain if already present.

Do not log:

- tokens
- secrets
- authorization headers
- cookies
- passwords
- customer/payment secrets

---

## 10. Validation Commands

Run focused tests first.

Then, if practical:

```bash
go test ./clients/... -count=1
go vet ./clients/...
git diff --check
```

If sqlc generation is required, run the repository's normal generation command and inspect the diff.

If generation exceeds the available command timeout:

- do not make unrelated generated changes;
- inspect/revert unrelated generator metadata churn;
- report the limitation.

---

## 11. Dashboard Verification

Do not modify Dashboard source unless the actual backend response proves there is a separate Dashboard defect.

Expected existing contract:

```text
backend row.name
    ->
Dashboard row.name
```

The purpose of this task is to make the backend endpoint return successfully.

---

## 12. Documentation

Append the result to existing checkpoint files.

At minimum update:

- root `.clinecheck`
- `clients/.clinecheck.md`
- `clients/.service-checkpoint.md`

Update Dashboard checkpoint files only if actual Dashboard runtime verification materially changes their status.

Do not overwrite existing entries.

Document:

- confirmed NULL-scan root cause;
- exact fix;
- affected query/repository;
- tests;
- runtime result;
- fallback behavior;
- remaining limitations.

Do not add this task to future Cline context.

---

## 13. Final Report

Return exactly these sections:

### 1. Root Cause

State the exact query/type involved and that:

```text
ClientRepo.ListSubAccounts attempted to scan nullable
clients.display_name into a non-null Go string.
```

### 2. Fix Implemented

List:

- exact file(s);
- exact query/repository change;
- generated code changes, if any.

### 3. Tests

List exact commands and results.

Explicitly include:

- NULL display-name test;
- populated display-name test;
- multiple-account test;
- gateway test if added/run.

### 4. Runtime Verification

State:

- Clients health status;
- actual Sub-Accounts HTTP status;
- number of accounts returned;
- friendly-name result;
- NULL fallback result;
- Dashboard result.

Do not claim runtime success unless actually verified.

### 5. Documentation Updated

List checkpoint files updated.

### 6. Remaining Issues

Only evidence-supported issues.

---

# Success Criteria

Complete when:

1. `ClientRepo.ListSubAccounts` no longer fails when `clients.display_name IS NULL`.
2. Existing friendly display names are preserved.
3. NULL display names fall back to `client_name` / `highlevel-<locationId>`.
4. Multiple accounts remain in the response.
5. Relevant tests pass.
6. The real authenticated Sub-Accounts request returns HTTP 200, if runtime verification is available.
7. No unrelated functionality or infrastructure is changed.

### Critical restriction

**Do not fix this by populating the database's NULL `display_name` values.**

Fix the application/query handling of the nullable field.

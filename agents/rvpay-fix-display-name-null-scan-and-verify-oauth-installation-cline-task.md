# RVPay — Implementation + End-to-End Verification: Fix `display_name` NULL Scan Regression

## Objective

Implement the confirmed `clients.display_name` NULL-scan fix currently breaking the HighLevel OAuth installation flow, then verify it with tests and, where the environment permits, a real end-to-end HighLevel installation.

The confirmed root cause is:

```text
clients.display_name is nullable
        ↓
sqlc-generated Go model uses string
        ↓
CreateClient / other queries return NULL display_name
        ↓
pgx cannot scan NULL into *string
        ↓
raw repository error
        ↓
translateError()
        ↓
gRPC Internal: "internal error"
        ↓
OAuth callback HTTP 500
        ↓
HighLevel: "OAuth Callback processing failed"
```

The failing installation was associated with HighLevel location:

```text
t18WTYAIBHPz2uHtbcvR
```

### Critical database context

**The local database available to this development environment is NOT the production/testing ECS database.**

Therefore:

- The local database is not expected to contain `t18WTYAIBHPz2uHtbcvR`.
- A search of the local database for that location ID may correctly return zero rows.
- Do NOT interpret absence of that location ID locally as evidence that the production/testing installation did not create an orphan row.
- Do NOT create fake local records for that production/testing location merely to make the query return something.
- Do NOT delete or modify local records simply because the production/testing location is absent.
- Do NOT claim the production/testing orphan-row question has been resolved using the local DB.
- Actual production/testing DB cleanup/query is a separate operational concern unless an explicitly configured and authorized read-only connection to that environment is genuinely available.

The implementation must be locally testable with synthetic/local records having `display_name = NULL`.

---

# Required Control-Document Reading

Before modifying anything, read:

### Root

```text
.clinecheck
.clinerules
.clineignore
README.md
```

### Clients

```text
clients/.service-checkpoint.md
clients/.clinerules.md
clients/.clineignore.md
clients/.clinecheck.md
clients/README.md
```

If a required file does not exist, report that fact.

Do not add this task file to project context.
Do not overwrite existing checkpoint/documentation content. Append only.

---

# Confirmed Diagnostic Findings

The previous diagnostic established:

- OAuth callback is reached.
- Empty `state` is an intentional stateless fallback and is not the cause.
- HighLevel token exchange succeeds.
- Location ID `t18WTYAIBHPz2uHtbcvR` is successfully obtained.
- Failure occurs after token exchange and before OAuth credential persistence.
- `CreateClient` is the first failing persistence operation for a genuinely new location.
- Migration `000006_client_display_name` introduced nullable `clients.display_name`.
- Generated sqlc code scans `display_name` into non-nullable Go `string`.
- `CreateClient` inserts without `display_name`, then its `RETURNING` result contains NULL.
- pgx fails scanning NULL into `*string`.
- The raw pgx error is ultimately converted into generic gRPC `Internal`.
- A similar NULL scan failure was previously observed in `ListSubAccountsFiltered`.
- The previous fix only addressed the listing query and did not cover all client repository queries.

Relevant historical commits identified by the diagnostic:

```text
50f0cbe — migration 000006_client_display_name
1846391 — ListSubAccountsFiltered COALESCE fix
```

Do not revert either commit.

---

# Scope of This Task

Implement the smallest correct repository/sqlc fix so nullable `display_name` can safely coexist with the existing non-nullable Go string representation.

The intended strategy is to apply the established query-boundary fallback pattern where appropriate:

```sql
COALESCE(display_name, client_name)
```

This means:

- database `display_name` remains nullable;
- existing NULL records remain valid;
- Go consumers receive a usable non-null display value;
- existing records without a friendly name fall back to `client_name`;
- new OAuth-created clients can be returned safely;
- no schema migration is required merely to eliminate NULL values.

**Inspect every affected SQL statement before changing it. Do not blindly replace every occurrence of `display_name`.**

---

# Required Investigation Before Implementation

Inspect the actual current SQL and generated code.

At minimum inspect:

```text
CreateClient
GetClientByID
GetClientByName
ListClients
ListActiveClients
UpdateClientStatus
UpdateClientDisplayName
```

Also search the entire `clients` service for other SQL queries that:

- return `display_name`;
- scan `display_name`;
- use `RETURNING *`;
- use `RETURNING` lists containing `display_name`;
- feed rows into a Go type where `DisplayName` is `string`.

Do not assume the diagnostic list is exhaustive.

Goal:

> No repository query that returns nullable `clients.display_name` should be capable of causing a NULL-to-string pgx scan failure.

---

# Implementation Requirements

## 1. Fix at the SQL boundary

Prefer the existing established pattern:

```sql
COALESCE(display_name, client_name)
```

where a query returns a value into the non-nullable Go representation.

For example, if appropriate:

```sql
RETURNING
    id,
    client_name,
    status,
    created_at,
    updated_at,
    COALESCE(display_name, client_name) AS display_name
```

Do not copy this blindly. Preserve each query's existing column order and aliases.

## 2. Preserve friendly names

Fallback applies only when `display_name IS NULL`.

For example:

```text
display_name = "RVPay Test Location"
```

must remain that value, while:

```text
display_name = NULL
client_name = "highlevel-abc123"
```

must return `highlevel-abc123` to the Go consumer.

Do not overwrite database `display_name` values.

## 3. Do not make the database column non-null

Do NOT:

- change `display_name` to `NOT NULL`;
- add a default that changes existing semantics;
- create a migration solely to populate names;
- backfill the entire table;
- alter existing friendly names.

---

# sqlc Requirements

After modifying SQL:

1. Determine the repository's documented sqlc generation command.
2. Regenerate sqlc using the project's normal process.
3. Inspect the generated diff.
4. Confirm generated types remain compatible with application code.
5. Do not hand-edit generated sqlc files if regeneration is available.

If regeneration is unavailable, document why and do not fabricate generated output.

---

# Regression Tests

Add or update targeted tests proving the actual defect is fixed.

Required scenarios:

### A — NULL display name

A client has `display_name = NULL`. A repository query must succeed and return `client_name` rather than a scan error.

### B — Non-NULL display name

A client has a friendly name. It must remain unchanged.

### C — CreateClient

Creating a client without explicitly setting `display_name` must succeed. This is the primary OAuth regression case.

### D — Existing client lookup

An existing client whose `display_name` is NULL must be retrievable without error, especially through the lookup used by OAuth installation (`GetClientByName` or the actual equivalent).

### E — Multiple clients

Where practical, cover NULL and non-NULL display names in the same result set.

### F — OAuth callback path

If existing test architecture makes it practical, cover the path where `GetClientByName` returns not found and `CreateClient` is called, proving the NULL scan no longer aborts the callback.

Do not create a new testing framework.

---

# Local Database Testing

Use the local DB only for local/synthetic regression testing.

Synthetic values may look like:

```text
client_name = "highlevel-test-null-display"
display_name = NULL
```

Do NOT expect this local DB to contain:

```text
t18WTYAIBHPz2uHtbcvR
```

If it does not, that is expected.

Do not fabricate a production/testing record locally.

If DB-backed tests require `CLIENTS_TEST_DATABASE_URL`, use the repository's normal configuration. If unavailable, report the skip honestly.

---

# Production/Testing Orphan-Row Check

The diagnostic noted that `CreateClient` may execute the INSERT before the pgx scan fails. A production/testing orphan client may therefore exist for:

```text
highlevel-t18WTYAIBHPz2uHtbcvR
```

However, the local DB is **not** authoritative for this question.

If an explicitly authorized read-only production/testing DB connection is genuinely available, inspect (read-only):

- client row
- display_name
- integration
- OAuth credential
- provider configuration

Do not delete or modify records.

If no production/testing DB access exists, report exactly:

```text
Production/testing orphan-row status could not be verified from the local database.
```

Do not infer production state from local absence.

---

# OAuth End-to-End Verification

After implementation and local tests, attempt real end-to-end verification if the environment provides the required testing deployment and HighLevel access.

Required sequence:

```text
Fresh HighLevel installation
        ↓
OAuth callback reaches RVPay
        ↓
state handling succeeds
        ↓
HighLevel token exchange succeeds
        ↓
client resolution succeeds
        ↓
CreateClient succeeds
        ↓
integration creation succeeds
        ↓
OAuth credential persistence succeeds
        ↓
provider registration/configuration succeeds
        ↓
callback returns success
```

Then verify the newly installed sub-account appears in the Clients/Sub-Accounts listing.

Never claim runtime success without actually performing the installation.

---

# Verification Levels

Clearly distinguish:

## Level 1 — Local static/unit verification

Examples:

```bash
go test ./clients/...
go vet ./clients/...
```

## Level 2 — Local DB-backed verification

Use synthetic clients and NULL `display_name` values. This proves the SQL/repository regression locally but says nothing about `t18WTYAIBHPz2uHtbcvR`.

## Level 3 — Real HighLevel/testing-server verification

Requires actual testing deployment and HighLevel access. Only this can prove the original installation incident is fixed end-to-end.

Label which levels were completed.

---

# Do Not Change OAuth Behavior

This is a confirmed SQL/data-access regression.

Do NOT change:

- OAuth authorization URL
- token URL
- OAuth v3 headers
- OAuth request field names
- `userType=Location`
- OAuth scopes
- state handling
- callback URLs
- HighLevel provider protocol

If a separate OAuth issue is discovered, stop and report it rather than expanding scope.

---

# Do Not Change These Areas

Do not modify:

```text
transactions/
admindashboard/
webhooks
CloudFormation
ECS configuration
deployment infrastructure
ports/listeners
PawaPay integration
```

Do not modify the existing Sub-Accounts HTTP/API contract.

---

# Error Handling

Do not add broad error-handling changes.

Do not alter `translateError` merely to expose this incident. The NULL scan should disappear at the SQL boundary.

---

# Runtime Logging

Do not add diagnostic logging unless absolutely necessary for runtime verification.

If temporary logging is required:

- it must be secret-safe;
- never log auth codes, access/refresh tokens, client secrets, Authorization headers, or cookies;
- remove unnecessary temporary logging before completion.

---

# Git / Change Discipline

Before changes:

```bash
git status
git branch --show-current
git log -5 --oneline
```

After changes:

```bash
git diff --stat
git diff
git status
```

Ensure no unrelated modifications are included. Do not revert unrelated user work.

---

# Required Verification Commands

Use documented repository commands where available. At minimum run relevant commands such as:

```bash
go test ./clients/...
go vet ./clients/...
git diff --check
```

Use the normal sqlc generation command.

If DB-backed tests are skipped because `CLIENTS_TEST_DATABASE_URL` is unavailable, state that explicitly.

---

# Documentation

Append an implementation/verification entry to:

```text
clients/.service-checkpoint.md
clients/.clinecheck.md
```

Do not overwrite existing content.

Include:

- confirmed root cause;
- implementation;
- affected SQL/query coverage;
- sqlc regeneration;
- tests;
- local DB verification;
- production/testing DB verification status;
- HighLevel runtime verification status;
- orphan-row uncertainty;
- commit/hash if applicable.

Keep it concise.

---

# Final Report

Use exactly these sections:

## 1. Implementation Summary

## 2. SQL / Repository Coverage

List every affected query and how it was made NULL-safe. State whether additional `display_name` queries were found.

## 3. sqlc Regeneration

State command, result, and generated files changed.

## 4. Regression Tests

Report NULL display name, non-NULL display name, CreateClient, existing lookup, multiple clients, and OAuth path where covered.

## 5. Local Database Verification

State whether local DB-backed verification was performed. Explicitly state that absence of `t18WTYAIBHPz2uHtbcvR` locally is expected.

## 6. Production/Testing Database Verification

State either:

```text
Verified via authorized read-only access
```

or:

```text
Not verified — local DB is not the production/testing DB.
```

## 7. HighLevel End-to-End Verification

State either:

```text
Verified — fresh installation succeeded
```

or:

```text
Not performed — environment did not provide the required HighLevel/testing deployment access.
```

Never claim success without a real installation.

## 8. Sub-Accounts Verification

If a real installation succeeds, verify the new location appears in the listing and report it.

## 9. Files Changed

Separate:

```text
Source
Tests
Generated
Documentation
```

## 10. Tests / Commands

List exact commands and outcomes.

## 11. Remaining Issues

Explicitly identify production/testing orphan-row status, anything not runtime verified, and unrelated issues discovered but not changed.

## 12. Commit

If committed, provide commit hash and message.

---

# Acceptance Criteria

This task is complete only when:

### Code

- All relevant nullable `display_name` query boundaries are NULL-safe.
- Friendly names are preserved.
- NULL values fall back to `client_name`.
- No schema migration is introduced merely to remove NULLs.
- No unrelated services are changed.

### Tests

- CreateClient no longer fails because `display_name` is NULL.
- Existing NULL-name clients can be read.
- Friendly names remain intact.
- Relevant tests pass.

### sqlc

- SQL changes are regenerated using the normal repository process.
- Generated code is consistent with SQL.

### Runtime

If testing deployment and HighLevel access are available:

- fresh HighLevel installation succeeds;
- OAuth callback returns success;
- client/integration/credential/provider setup completes;
- new sub-account appears in listing.

If unavailable, explicitly report runtime verification as unavailable rather than simulating it.

### Database

- Local DB is used only for local/synthetic verification.
- No assumption is made that `t18WTYAIBHPz2uHtbcvR` exists locally.
- No production/testing data is modified.
- Production/testing orphan-row status is clearly separated from local DB results.

### Documentation

- Existing checkpoint files are appended, not overwritten.
- Final status distinguishes implemented, executed, and verified.

**Do not broaden the task. Fix the confirmed NULL-scan regression and verify it as far end-to-end as the available environment legitimately allows.**

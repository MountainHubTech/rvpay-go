# Cline Diagnostic Task — Admin Dashboard Account/Customer Names + Sub-Accounts 500

## Task Type

**Diagnostic only. Do NOT modify application source code, protobufs, SQL, migrations, generated code, configuration, or documentation.**

Investigate why the local Admin Dashboard currently:
1. Returns HTTP 500 when loading Sub-Accounts.
2. Still displays `highlevel-<locationId>` instead of the human-readable HighLevel sub-account name.
3. Still displays the internal customer ID instead of the human-readable customer name.

Produce a **precise, concise diagnostic report** identifying exact root causes, affected files/lines, and the smallest corrective changes a future implementation agent should make.

Do not implement fixes.

---

# 1. Mission

Determine exactly why the recent backend account/customer-name implementation is not reflected correctly in the Admin Dashboard.

Distinguish between:
- Dashboard bugs/missing changes
- Clients-service bugs
- Transactions-service bugs
- Missing local DB migrations
- Protobuf/generated-code/API contract mismatches
- Missing runtime data/backfill
- Configuration/environment problems
- Multiple independent causes

Do not assume the Dashboard is at fault merely because IDs are displayed. Do not assume the backend is correct merely because tests passed.

Use the actual current repository state and local runtime behavior.

---

# 2. Critical Constraints

### 2.1 Diagnostic only

Do NOT:
- edit `.tsx`, `.ts`, `.go`, `.proto`, `.sql`, `.yaml`, `.json`, or generated files;
- apply migrations;
- change environment variables;
- change database data;
- run production commands;
- commit anything;
- create fixes;
- regenerate protobuf/sqlc code;
- modify documentation/checkpoint files.

You MAY:
- read files;
- inspect git status/diff;
- search source;
- run existing tests;
- run safe read-only build/type-check/lint commands;
- inspect local logs;
- inspect local HTTP responses;
- inspect database schema/data using strictly read-only SQL if the local DB connection is already configured;
- use `git diff`, `git status`, `git log`, `git show`, `git blame`.

If a command could modify state, do not run it.

### 2.2 Context minimization

Do not read the entire repository indiscriminately.

Start with the specific files/searches below. Expand only when evidence requires it.

Do not spend tokens explaining generic framework behavior.

### 2.3 Previous agent report is not proof

The previous Cline report claimed the account/customer-name task was complete, but local behavior contradicts that claim.

Treat that report as history/hypothesis only. Verify claims against current source and runtime.

---

# 3. Mandatory Repository Context

Read these project-control files first:

- `.clinecheck`
- `.clinerules`
- `.clineignore`
- `README.md`

Admin Dashboard:
- `admindashboard/.project-checkpoint.md`
- `admindashboard/.clinerules.md`
- `admindashboard/.clineignore.md`
- `admindashboard/.clinecheck.md`
- `admindashboard/README.md`

Clients:
- `clients/.service-checkpoint.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/README.md`

Transactions:
- `transactions/.service-checkpoint.md`
- `transactions/.clinerules.md`
- `transactions/.clineignore.md`
- `transactions/.clinecheck.md`
- `transactions/README.md`

Also locate and read the relevant sections of:
- `agents/rvpay-correct-account-customer-names-cline-agent.md`

Do not read unrelated repository content.

---

# 4. Git State

Run:

```bash
git status --short
git diff --stat
git diff --name-only
git log -5 --oneline
```

Determine:
1. Whether the account/customer-name implementation is committed or uncommitted.
2. Exactly which Admin Dashboard source files changed as part of that task.
3. Whether the previous implementation actually touched Dashboard source code.
4. Whether uncommitted changes could affect local behavior.

Report this explicitly.

---

# 5. Diagnose the Sub-Accounts HTTP 500

This is the highest-priority investigation.

## 5.1 Identify the exact Dashboard request

Search for:

```text
sub-accounts
subAccounts
ListSubAccounts
/v1/public/clients
client_name
clientName
SubAccountRow
```

Identify:
- exact frontend file;
- exact API path;
- HTTP method;
- query/request parameters;
- expected response shape;
- response mapping;
- UI rendering code.

## 5.2 Trace the backend

Follow the request through:

```text
Dashboard component
    ↓
Dashboard API client
    ↓
HTTP endpoint
    ↓
Clients HTTP handler
    ↓
Clients service/RPC
    ↓
Client repository
    ↓
SQL query
    ↓
Database
```

Identify exact files/functions at each relevant layer.

## 5.3 Determine the actual 500 cause

If services are already running, make the same read-only request the Dashboard makes.

Capture:
- HTTP status;
- response body;
- relevant Clients-service log lines;
- relevant Dashboard/server log lines.

If services are not running, do not start them automatically unless the documented local procedure is clearly non-destructive and necessary. If runtime reproduction is impossible, say so.

Pay special attention to:
- `display_name`
- `clients/db/migrations/000006_client_display_name.up.sql`

If read-only DB inspection is possible, verify whether `clients.display_name` exists.

**Do not apply the migration.**

Explicitly distinguish:
- malformed Dashboard request;
- correct request but Clients service failure;
- missing migration;
- SQL/repository bug;
- backend succeeds but Dashboard mishandles response;
- other concrete cause.

---

# 6. Diagnose Account/Sub-Account Name Rendering

Inspect the actual Dashboard rendering after understanding the request path.

Determine exactly what field is displayed.

Search for:
```text
row.name
row.clientName
row.client_name
row.id
row.locationId
row.externalAccountId
```

Intended backend contract:

```text
SubAccountRow.name
    = human-readable HighLevel location/sub-account name

SubAccountRow.client_name
    = internal RVPay identifier, e.g. highlevel-<locationId>
```

Determine whether the Dashboard:
1. consumes `name`;
2. consumes the identifier;
3. expects `clientName` while backend exposes `client_name`;
4. has stale generated/API types;
5. transforms the response incorrectly;
6. never receives the response because of the 500.

Report exact file/function/line where the wrong field is selected.

Do not fix it.

---

# 7. Diagnose Transaction Customer Name

Trace:

```text
Dashboard transaction component
    ↓
Dashboard API call
    ↓
Transactions endpoint
    ↓
Transactions handler/service
    ↓
TransactionListRow
    ↓
protobuf/API response
    ↓
Dashboard rendering
```

Search for:
```text
transactions
customer
customerName
customer_name
TransactionListRow
clientName
subAccount
```

Intended contract:

```text
TransactionListRow.customer
    = internal customer ID

TransactionListRow.customer_name
    = human-readable customer name
```

Determine whether the problem is:
- Dashboard never reads `customer_name`;
- Dashboard API/generated type lacks it;
- backend returns it empty;
- Transactions service never resolves/persists it;
- lookup exists but is not used in transaction listing;
- customer rows have NULL/empty names;
- transaction initiation never supplied a name;
- historical transactions have no recoverable name;
- other.

Report exact evidence.

---

# 8. Verify Backend Contract Independently

Do not assume protobuf fields are populated.

Inspect only relevant portions of:
- `protobuf/clients.proto`
- generated clients protobuf code
- `clients/service/converters.go`
- `clients/service/clients_service.go`
- `clients/db/query/clients.sql`
- `clients/db/repo/client_repo.go`
- relevant OAuth/name-sync files
- `protobuf/transactions.proto`
- generated transactions protobuf code
- transaction listing service/handler/converters
- customer repository/SQL
- relevant payment-service changes

Determine:

```text
Does ListSubAccounts actually populate SubAccountRow.name?
Does it populate SubAccountRow.client_name?
Does ListClients populate Client.display_name?
Does the transaction list actually populate customer_name?
```

---

# 9. Verify Database Migration State

Inspect:

```text
clients/db/migrations/000006_client_display_name.up.sql
```

Determine:
- whether migration exists;
- what schema it changes;
- whether local migration tooling would apply it;
- what the local DB schema actually contains, if safely inspectable.

Do NOT run the migration.

If code references `display_name` but local DB lacks it, identify that as the root cause of the 500.

---

# 10. Verify Generated/API Contract

Determine whether the Dashboard uses:
- generated gRPC code;
- generated TypeScript types;
- manually defined TS interfaces;
- JSON mapping;
- REST gateway;
- custom API adapters.

Verify whether the relevant structures contain:
- `display_name`
- `client_name`
- `customer_name`

Pay special attention to snake_case vs camelCase. Find the actual runtime response shape rather than guessing.

---

# 11. Check Local Data

For accounts, determine whether existing `clients` rows contain `display_name` and whether values are NULL/empty.

For customers, determine whether relevant `customers.name` values exist.

Use only read-only SQL if the local DB is already accessible.

Do not modify rows.

Only report measured counts. If DB access is unavailable, say:

```text
Local database contents could not be verified from this environment.
```

Do not invent counts.

---

# 12. Determine Whether Backfill Was Actually Run

Determine whether:

```bash
go run ./clients/cmd/backfill-client-names
```

was actually executed.

Do NOT execute it.

Distinguish:
- implementation exists;
- implementation was executed;
- data was verified.

These are different facts.

---

# 13. HighLevel Scope / Runtime Name Resolution

Verify the code path requiring:

```text
locations.readonly
```

Determine:
- whether the scope is in current OAuth configuration;
- whether existing local tokens would have the new scope;
- whether enrichment fails safely without it;
- whether that can explain NULL `display_name`.

Do NOT reauthorize OAuth or contact HighLevel.

Never print access tokens, client secrets, or other credentials.

---

# 14. Backend vs Dashboard Contract Mapping

Produce this exact mapping:

```text
Backend field                    Dashboard field
-------------------------------------------------
SubAccountRow.name               ?
SubAccountRow.client_name        ?
TransactionListRow.customer      ?
TransactionListRow.customer_name ?
Client.display_name              ?
```

For each:
- exact frontend file;
- exact field used;
- whether correct;
- whether transformation occurs.

---

# 15. Do Not Fix Anything

Do NOT:
- modify source;
- modify database;
- apply migrations;
- change configuration;
- update documentation;
- regenerate generated files;
- create a branch;
- commit changes.

At the end, `git diff` must show the same state as at the beginning.

---

# 16. Required Final Report

Use exactly this structure:

# Diagnostic Report — Account/Customer Names + Sub-Accounts 500

## 1. Executive Finding

Maximum 5 bullets.

State actual root causes, not theories.

Use categories such as:
- `Sub-Accounts 500: ...`
- `Account name display: ...`
- `Customer name display: ...`
- `Data/backfill: ...`
- `Dashboard changes required: YES/NO`

## 2. Sub-Accounts 500

```text
Request:
HTTP status:
Actual error:
Root cause:
Evidence:
Affected file(s):
```

## 3. Account/Sub-Account Name

```text
Backend returns:
Dashboard reads:
Expected field:
Actual field:
Root cause:
Affected file(s):
```

## 4. Customer Name

```text
Backend returns:
Dashboard reads:
Expected field:
Actual field:
Root cause:
Affected file(s):
```

## 5. Data/Migration Status

```text
display_name column exists locally: YES/NO/UNKNOWN
display_name data populated: YES/NO/UNKNOWN
customer names populated: YES/NO/UNKNOWN
client-name backfill executed: YES/NO/UNKNOWN
customer-name backfill executed: YES/NO/UNKNOWN
```

Only use YES/NO when verified.

## 6. Dashboard Changes Required

```text
Dashboard source changes required: YES/NO
```

If YES, list only exact files and what each needs to change. Do not implement.

## 7. Backend Changes Required

```text
Backend source changes required: YES/NO
```

If YES, list only exact files and what each needs to change.

## 8. Migration/Runtime Action Required

List only actions actually required, for example:
- Apply migration 000006 locally.
- Re-authorize HighLevel installation with `locations.readonly`.
- Run client-name backfill.

Do not perform them.

## 9. Confidence / Limitations

Maximum 3 bullets.

State anything that could not be verified.

---

# 17. Final Safety Check

Run:

```bash
git status --short
git diff --stat
git diff --name-only
```

Confirm:

```text
Repository modifications made by this diagnostic: NONE
```

If anything changed unexpectedly, report it precisely.

---

# Success Criteria

The final report must answer, without another investigation:

1. Why does `/v1/public/clients/sub-accounts` return 500?
2. Is the 500 caused by Dashboard, Clients service, SQL, or missing migration?
3. Does backend actually return the human-readable account name?
4. Which exact Dashboard field is rendered for sub-account name?
5. Does backend actually return `customer_name`?
6. Which exact Dashboard field is rendered for customer name?
7. Are local account names populated?
8. Are local customer names populated?
9. Was backfill actually executed?
10. Exactly which files need changing?
11. Which changes are Dashboard-only versus backend/data/migration changes?

**Do not provide general recommendations. Provide evidence and exact findings.**

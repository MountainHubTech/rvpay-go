# Cline Agent Task — Audit and Implement Transactions-by-Sub-Account Endpoint

## Objective

Determine whether RVPay already exposes an endpoint that lists transactions belonging to a specific GHL sub-account/client.

The endpoint must be capable of identifying the sub-account by either:

1. the RVPay registered client/sub-account database ID; or
2. the GHL `locationId` associated with that client.

If an appropriate endpoint already exists and is correctly exposed, **do not create a duplicate endpoint**. Audit and verify it.

If no appropriate endpoint exists, implement one end-to-end, including database/query support, repository/service support, API exposure, tests, and documentation/checkpoint updates.

This task is specifically about **listing transactions by sub-account/client**.

Do NOT modify the Dashboard, custom GHL transaction page, OAuth installation flow, payment-provider configuration, commission logic, or unrelated transaction behavior.

---

# 1. Required repository context

Before inspecting or changing code, read these files completely.

## Root/project context

Read:

- `.clinecheck`
- `.clinerules`
- `.clineignore`
- `README.md`

Also read the current root project checkpoint if one exists and is referenced by the above files.

## Transactions service

Read:

- `transactions/.service-checkpoint.md`
- `transactions/.clinerules.md`
- `transactions/.clineignore.md`
- `transactions/.clinecheck.md`
- `transactions/README.md`

Then inspect the Transactions source tree to identify the actual:

- database schema/migrations;
- SQL query files;
- sqlc configuration;
- repository implementation;
- service implementation;
- protobuf definitions;
- generated gRPC code;
- HTTP handlers/routes/gateway;
- existing transaction-list endpoints;
- transaction tests.

## Clients service

Read:

- `clients/.service-checkpoint.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/README.md`

Then inspect the current Clients schema/model/repository code that establishes how a GHL sub-account is represented and identified.

Specifically determine:

- the internal client/sub-account primary key;
- the field containing the GHL `locationId`;
- whether that field is currently `external_account_id` or another field;
- how Clients identifies a HighLevel location;
- how Transactions currently refers to a client.

Do NOT assume field names from this task. Confirm them from the repository.

---

# 2. Audit first — do not immediately implement

Trace the existing transaction-listing flow end-to-end:

HTTP endpoint
→ HTTP handler/gateway
→ gRPC/API request
→ Transactions service
→ repository
→ SQL/sqlc query
→ database.

Determine whether an existing endpoint can answer:

> Give me the transactions belonging to this specific RVPay client/sub-account.

Search for:

- transaction list handlers;
- `ListTransactions`;
- filtered transaction list methods;
- `client_id`;
- `external_account_id`;
- `location_id`;
- `locationId`;
- client/sub-account filters;
- transaction filtering SQL;
- admin transaction APIs;
- public transaction APIs;
- gRPC request fields.

Do not create a duplicate endpoint if an existing endpoint can be appropriately extended.

---

# 3. Establish the authoritative transaction relationship

Determine exactly how a transaction is currently associated with a sub-account.

Confirm whether the relationship is something such as:

`transactions.client_id → clients.id`

or another existing relationship.

Also determine whether Transactions stores the GHL `locationId` directly or only stores the RVPay client ID.

Prefer the existing canonical relationship:

GHL locationId
→ Clients location/client record
→ RVPay client ID
→ Transactions
→ transaction records

Do NOT duplicate `locationId` into Transactions merely to make this endpoint easier unless the existing architecture genuinely requires it.

Do not introduce an unnecessary cross-service database dependency.

---

# 4. Decide whether to reuse or create the endpoint

Make exactly one determination.

## Case A — Existing endpoint is sufficient

If an endpoint already lists transactions by sub-account/client:

- do not create another endpoint;
- verify client-ID filtering;
- verify locationId filtering if already supported;
- verify the endpoint is actually HTTP-exposed;
- verify request parameters reach the repository;
- verify SQL performs server-side filtering;
- verify pagination/count/order remain correct;
- verify unrelated transactions are excluded.

## Case B — Existing endpoint is incomplete

Extend the existing endpoint.

Do not create a second endpoint.

Document exactly what was missing and what was changed.

## Case C — No endpoint exists

Implement a new endpoint following existing Transactions API conventions.

---

# 5. Required API behavior

The final API must support identifying a sub-account using the available authoritative identifier(s):

### RVPay client ID

Conceptually:

`GET /<existing-transactions-route>?clientId=<rvpay-client-id>`

### GHL locationId

Conceptually:

`GET /<existing-transactions-route>?locationId=<ghl-location-id>`

Use the repository's actual route and naming conventions. Do not invent a new API style.

If both identifiers are accepted:

- verify they refer to the same client;
- reject conflicting identifiers rather than silently choosing one;
- preserve existing pagination and transaction filters.

If only one identifier can safely be supported because of the current architecture, document why and implement the other through the correct existing Clients lookup path rather than duplicating data.

---

# 6. Database/query implementation

If repository support is missing:

- add/modify SQL in the existing Transactions query directory;
- follow the existing SQL/sqlc conventions;
- do not put SQL in handlers/services.

If sqlc is used:

1. modify the SQL query;
2. regenerate sqlc;
3. commit generated code;
4. never manually edit generated sqlc files.

First inspect the actual generation mechanism:

- `sqlc.yaml` / `sqlc.yml`;
- `Makefile`;
- `go:generate` directives.

Use the repository's established command, such as:

`make generate`

or:

`go generate ./...`

Do not invent a new generation process.

The query must:

- filter by the canonical client/sub-account relationship;
- return the existing transaction-list response shape;
- preserve existing ordering;
- preserve pagination;
- preserve total/count behavior if the current API provides it.

---

# 7. Repository and service implementation

If support is missing/incomplete:

Implement the repository method/query and the Transactions service method in the existing architecture.

The service must:

- validate supplied identifier(s);
- resolve locationId to the correct RVPay client when necessary;
- pass the canonical client ID to the repository;
- preserve existing filters/pagination;
- use existing error conventions.

If protobuf/gRPC is used:

1. modify the appropriate `.proto`;
2. add the required request fields/RPC changes;
3. regenerate protobuf/gRPC code using the repository's existing generation command;
4. never hand-edit generated files.

---

# 8. HTTP/API exposure

If the capability is not exposed:

Expose it using the existing Transactions HTTP/API conventions.

Inspect existing routes before choosing the route.

The endpoint must correctly map the supported identifier(s) and existing pagination fields.

Do not create a new top-level API namespace.

---

# 9. Tests

Tests are mandatory.

## Repository/database tests

Cover at minimum:

### Client ID

- returns transactions for requested client;
- does not return another client's transactions;
- returns empty result when client has no transactions.

### Location ID

Where locationId resolution is supported:

- valid locationId resolves to correct client;
- unknown locationId returns the expected not-found/validation result.

### Multiple clients

Create at least:

- Client A;
- Client B;
- Client C;

with transactions for each.

Verify each query returns only its own client's transactions.

### Pagination

Verify:

- page 1;
- page 2;
- page size;
- total count.

### Ordering

Verify existing deterministic transaction ordering remains intact.

## API/handler tests

Cover:

- valid clientId;
- valid locationId;
- missing identifier;
- malformed identifier;
- unknown client;
- unknown location;
- conflicting clientId/locationId if both are supported;
- pagination;
- repository/service error;
- successful response serialization.

---

# 10. Mandatory tenant-isolation test

Explicitly prove that server-side filtering prevents cross-sub-account leakage.

Example:

Request:

`clientId = Client A`

Expected:

- Transaction A1
- Transaction A2

Must NOT contain:

- Transaction B1
- Transaction B2
- Transaction C1
- Transaction C2

For locationId:

`locationId = Location A`

must return only transactions belonging to Location A's RVPay client.

Do not rely on frontend filtering.

---

# 11. Scope restrictions

Do NOT modify:

- Dashboard frontend;
- custom GHL transaction page;
- OAuth installation flow;
- agency implementation;
- payment-provider configuration;
- PawaPay integration;
- commission logic;
- webhook behavior;
- AWS infrastructure;
- CloudFormation;
- production ports;
- `transactions/main.go` port configuration.

The existing local `8081` workaround is not part of this task and must not be made permanent.

---

# 12. Documentation/checkpoint updates

Append to the existing documentation; never overwrite historical content.

## Transactions

Update:

- `transactions/.service-checkpoint.md`
- `transactions/README.md`
- `transactions/.clinecheck.md`

only where appropriate under the existing documentation conventions.

Document:

- whether an endpoint already existed;
- whether it was reused, extended, or newly created;
- exact endpoint;
- supported identifier(s);
- SQL/query changes;
- sqlc generation;
- protobuf/API changes;
- tests added;
- verification performed.

## Clients

If the implementation required a Clients-side lookup or API change, append the relevant change to:

- `clients/.service-checkpoint.md`
- `clients/README.md`
- `clients/.clinecheck.md`

Do not modify Clients code merely to create a new abstraction if an existing lookup is sufficient.

Do not add a new task/context file.

Do not modify `.clinerules` or `.clineignore` unless genuinely required.

---

# 13. Verification

Run the repository's appropriate checks.

At minimum, where applicable:

`go test ./transactions/...`

`go vet ./transactions/...`

If Clients code was changed:

`go test ./clients/...`

`go vet ./clients/...`

Also run:

`git diff --check`

If database-backed tests require an environment variable such as `TRANSACTIONS_TEST_DATABASE_URL`:

- run them if available;
- otherwise explicitly report that they were skipped;
- never claim database-backed verification that did not happen.

Inspect the final diff for accidental unrelated changes.

---

# 14. Final report

Return a precise report with:

## A. Existing endpoint audit

State:

- whether an endpoint existed;
- exact route;
- HTTP method;
- existing filters;
- client-ID support;
- locationId support;
- whether it was exposed.

## B. Implementation result

State exactly one:

- `Existing endpoint verified — no implementation required`
- `Existing endpoint extended`
- `New endpoint created`

Then list exact implementation changes.

## C. Database/query changes

List:

- SQL files;
- queries;
- sqlc generation command;
- generated files.

## D. API changes

List:

- protobuf changes;
- gRPC changes;
- HTTP route;
- request parameters;
- response behavior.

## E. Tests

Separate:

- repository/database tests;
- service tests;
- HTTP/API tests;
- integration tests.

Mark each PASS or SKIPPED with reason.

## F. Documentation

List every checkpoint/README file updated and confirm existing content was appended rather than overwritten.

## G. Scope verification

Confirm Dashboard, OAuth, payment providers, PawaPay, commission, webhooks, infrastructure, and production ports were not modified.

## H. Remaining issues

Only actual remaining issues.

## I. Git state

Report:

`git status --short`

`git diff --check`

current HEAD/commit.

Do not create a commit unless explicitly requested.

---

# Definition of Done

This task is complete only when:

1. Existing transaction-listing endpoints have been audited.
2. It is known whether sub-account transaction listing already exists.
3. No duplicate endpoint was created when an existing endpoint could be reused.
4. If necessary, the existing endpoint was extended or a new one was implemented.
5. The endpoint identifies the sub-account using the appropriate RVPay client ID and/or GHL locationId.
6. Filtering is performed server-side.
7. Cross-sub-account transaction leakage is explicitly tested.
8. SQL/sqlc/protobuf generation is correctly performed where applicable.
9. API exposure is verified.
10. Tests cover identifiers, isolation, pagination and errors.
11. Existing transaction behavior remains intact.
12. Required checkpoint/README documentation is appended.
13. No unrelated Phase 2 work was introduced.
14. The final report clearly distinguishes discovered, implemented, tested, and actually verified behavior.

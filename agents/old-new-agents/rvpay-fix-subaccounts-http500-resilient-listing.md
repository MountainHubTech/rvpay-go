# Cline Task — Fix Sub-Accounts HTTP 500 and Make Listing Resilient

## Objective

Diagnose and fix the **Sub-Accounts HTTP 500** in the RVPay Dashboard.

Required behavior:

> Every existing HighLevel sub-account/integration must be returned by the Sub-Accounts endpoint.

A friendly HighLevel location name is desirable, but it is **not required** for an account to appear.

If the friendly name cannot be obtained, the account must fall back to its existing deterministic identifier:

`highlevel-<locationId>`

Most importantly:

> A failure to enrich one sub-account's display name must never cause the entire Sub-Accounts list request to return HTTP 500.

This task is intentionally narrow.

Do NOT redesign OAuth.
Do NOT investigate the two-location credential lifecycle again unless the Sub-Accounts 500 directly requires it.
Do NOT touch Transactions customer-name handling.
Do NOT touch HighLevel native order sync.
Do NOT touch inbound webhook delivery.
Do NOT change CloudFormation, ECS listeners, production ports, or deployment infrastructure.

---

## 1. Current context

The Transactions Customer column now correctly displays actual customer names.

**Do not modify that path.**

The intended Sub-Accounts flow is:

`HighLevel integration/client row -> clients.display_name -> SubAccountRow.name -> Dashboard row.name`

with fallback to:

`highlevel-<locationId>`

The problem is that the Sub-Accounts page currently receives:

`HTTP 500 from clients`

instead of a list of accounts.

The testing environment contains **more HighLevel integrations than the two previously investigated location IDs**, and all existing integrations must remain visible.

A particular location lacking a locally stored credential is not, by itself, a reason to hide that account or fail the entire request.

---

## 2. Local port constraint

The user has confirmed that locally the Transactions service must currently be manually changed to port `8081` in `main.go` if both Clients and Transactions are run simultaneously.

This is only a local testing workaround.

Do NOT:
- make the port change permanent;
- modify CloudFormation;
- modify ECS listener configuration;
- modify production networking;
- spend time solving the port architecture.

If Transactions is not required to reproduce this Clients/Dashboard issue, do not start or modify it.

---

## 3. Mandatory repository context

Before changing code, read:

### Root
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
- `admindashboard/.project-checkpoint.md`
- `admindashboard/.clinerules.md`
- `admindashboard/.clineignore.md`
- `admindashboard/.clinecheck.md`
- `admindashboard/README.md`

Read other files only when directly relevant to the Sub-Accounts request path.

Do not add this task file to project context/checkpoint files.

Append documentation; never overwrite existing checkpoint content.

---

## 4. First priority: reproduce the actual HTTP 500

Do not start by changing code.

Run the actual Clients service and Dashboard path used for Sub-Accounts.

Reproduce:

`Dashboard -> Sub-Accounts -> Clients API -> HTTP 500`

Capture the actual server-side error.

Do not stop at "Dashboard says HTTP 500."

Identify:
- endpoint;
- handler;
- service method;
- repository/database operation;
- exact error message;
- relevant integration/location if identifiable;
- source location if available.

The final report must state the actual root failure.

---

## 5. Trace the complete Sub-Accounts path

Trace end-to-end:

`Dashboard Sub-Accounts page`
`-> Dashboard API client`
`-> HTTP request`
`-> Clients HTTP handler`
`-> Clients service method`
`-> database/repository query`
`-> SubAccountRow mapping`
`-> JSON/protobuf response`
`-> Dashboard response parsing`
`-> Sub-Accounts rendering`

Identify exactly where the HTTP 500 originates.

Do not assume the Dashboard is responsible merely because the page displays an error.

---

## 6. Enumerate ALL existing HighLevel integrations

Using the actual database used by the running Clients service, determine the complete set of existing HighLevel integrations/clients.

Do not restrict the investigation to:
- `lamyP5Es8Q2nQrb90fuv`
- `rVBsqXAKXlYLz0C96kA8`

Those two IDs are not the complete testing environment.

For every existing HighLevel integration, inspect at minimum:
- integration ID;
- external account/location ID;
- client name;
- display name;
- active/status state;
- credential presence if relevant;
- fields directly used by `ListSubAccountsFiltered`.

Do not print secrets.

The purpose is to determine whether one row or data condition causes the whole list operation to fail.

---

## 7. Determine whether the failure is data-dependent

Test whether the 500 is caused by one or more specific rows.

Look for:
- NULL values;
- missing display names;
- missing integration relationships;
- missing credentials;
- expired credentials;
- duplicate records;
- unexpected provider values;
- malformed location IDs;
- database constraints;
- scan errors;
- SQL query errors;
- protobuf mapping failures;
- JSON serialization errors;
- assumptions that every integration has an OAuth credential;
- assumptions that every integration can be enriched through HighLevel.

Do not "fix" the data merely to make the test pass.

The objective is to make the application robust to valid incomplete/partially enriched account data.

---

## 8. Critical behavioral requirement

The Sub-Accounts listing must behave like this:

For each existing HighLevel account:
1. try to obtain display name;
2. if display name is available, use it;
3. otherwise use `highlevel-<locationId>`;
4. return the account.

Conceptually:

- Account A -> name available -> `Account A`
- Account B -> name unavailable -> `highlevel-locationB`
- Account C -> name available -> `Account C`
- Account D -> enrichment fails -> `highlevel-locationD`

The response should still be:

`HTTP 200`

with all accounts.

Do not allow one account's enrichment failure to abort the entire collection.

---

## 9. Important distinction: listing vs enrichment

Treat these as separate operations.

### Required
List existing HighLevel integrations.

### Optional enrichment
Resolve friendly HighLevel location name.

A missing/failed enrichment must not imply:

`integration does not exist`

and must not cause:

`ListSubAccounts -> HTTP 500`

The existing integration/client row is sufficient to display the account using its fallback identifier.

---

## 10. Investigate the existing display-name implementation

Inspect the current implementation introduced for `clients.display_name`.

Determine:
1. where `display_name` is selected;
2. where `SubAccountRow.name` is constructed;
3. where fallback to `client_name` occurs;
4. whether fallback is reached for NULL/empty values;
5. whether any operation before fallback can throw an error;
6. whether display-name enrichment is performed during the list operation;
7. whether enrichment can fail the entire request;
8. whether a database scan can fail because `display_name` is NULL;
9. whether one malformed row aborts the query.

Do not replace the implementation blindly. Identify the exact failure first.

---

## 11. Investigate credentials only as a dependency

A credential may be necessary to obtain a friendly HighLevel location name.

However:

> A missing/expired/unusable credential must not prevent an existing account from being listed.

If the current list implementation calls HighLevel for every account, determine whether that is causing the 500.

If so, make enrichment best-effort:

`integration exists -> try location-name enrichment -> success: display_name -> failure: client_name fallback -> return account`

Do not redesign credential management.

Do not alter token refresh behavior unless absolutely required to stop the 500.

---

## 12. Preserve existing fallback semantics

The expected fallback remains:

`highlevel-<locationId>`

or the repository's established equivalent.

Do not introduce:
- blank names;
- `"Unknown"` unless already specified;
- random names;
- database IDs as user-facing names.

If `display_name` is absent, preserve the deterministic HighLevel location-based identifier.

---

## 13. Do not hide accounts

The following are NOT acceptable fixes:
- filtering out accounts with missing credentials;
- filtering out accounts with NULL `display_name`;
- filtering out accounts whose HighLevel API lookup fails;
- returning only successfully enriched accounts;
- silently skipping rows;
- catching an error and returning an incomplete list.

The endpoint must return all valid existing HighLevel integrations.

---

## 14. Dashboard verification

Only after the Clients endpoint is working, verify the Dashboard.

Confirm:
1. Sub-Accounts page makes the expected request;
2. HTTP status is 200;
3. response contains all existing HighLevel integrations;
4. accounts with `display_name` show the actual friendly name;
5. accounts without `display_name` show `highlevel-<locationId>`;
6. no account disappears because enrichment failed;
7. no client-side parsing/rendering error turns a valid backend response into an error state.

Do not modify Dashboard code unless the backend response is correct and there is an independently confirmed Dashboard defect.

---

## 15. Testing requirements

Add or update focused tests for the confirmed failure mode.

At minimum, where applicable:

### Normal account
`display_name present -> actual name returned`

### Missing display name
`display_name NULL/empty -> highlevel-<locationId>`

### Enrichment failure
`HighLevel name lookup fails -> account still returned -> fallback name used -> entire request remains successful`

### Multiple accounts
`Account A succeeds; Account B enrichment fails; Account C succeeds -> A, B, C all returned -> HTTP 200`

The multi-account case is especially important.

Do not claim the behavior is fixed without testing the case where one account fails while others succeed.

---

## 16. Runtime verification

Use the actual local database/environment currently configured for the Clients service.

Verify:
- the database being queried is the one the running service uses;
- the complete set of HighLevel integrations;
- endpoint behavior against that data.

If it is possible to reproduce the exact 500 locally, do so.

If the local dataset differs from the testing server and prevents exact reproduction, state that clearly.

Do not manufacture production/test data.

Do not modify production data.

---

## 17. Minimal implementation rule

Only make code changes after the failure is proven.

The preferred fix should establish this invariant:

> **Sub-Accounts listing is independent of optional display-name enrichment.**

Avoid:
- broad refactoring;
- new abstractions without need;
- OAuth redesign;
- schema redesign;
- API contract redesign;
- unrelated cleanup.

---

## 18. Documentation

Append the outcome to the appropriate existing checkpoint files.

Document:
- exact HTTP 500 root cause;
- affected operation;
- whether failure was caused by one account or the whole query;
- behavior before fix;
- behavior after fix;
- fallback behavior;
- tests added;
- runtime verification performed.

Do not overwrite existing checkpoint content.

---

## 19. Final report — mandatory format

End with exactly these sections:

### 1. Executive finding
Explain:
- why the Sub-Accounts endpoint returned HTTP 500;
- whether the failure was account-specific or systemic;
- what changed.

### 2. Before
Describe the actual failing path and include the relevant error.

### 3. After
Describe the corrected behavior.

Explicitly confirm, only if verified:
- HTTP 200;
- all existing HighLevel integrations returned;
- friendly names used where available;
- `highlevel-<locationId>` fallback used otherwise;
- one enrichment failure does not abort the list.

### 4. Root cause
State the exact confirmed root cause. No speculation.

### 5. Files changed
List every changed file and why.

### 6. Tests
List exact test commands and results.

### 7. Runtime verification
List exact runtime checks and results.

Clearly distinguish:
- implemented;
- executed;
- verified.

### 8. Remaining limitations
Only genuinely unverified items.

### 9. Next action
Give the smallest remaining action, if any.

---

## Definition of done

This task is complete only when evidence demonstrates that:

1. The actual Sub-Accounts HTTP 500 has been reproduced or its exact server-side cause established.
2. The failing operation is identified.
3. All existing HighLevel integrations can be returned.
4. `display_name` is used when available.
5. `highlevel-<locationId>` is used when it is unavailable.
6. A missing/expired/unusable credential does not cause an existing account to disappear.
7. A failed optional HighLevel name lookup does not cause the entire list to fail.
8. The endpoint returns HTTP 200 with all valid existing accounts.
9. The Dashboard successfully renders that response.
10. Focused tests cover the failure mode.
11. No unrelated OAuth, Transactions, webhook, CloudFormation, listener, or port changes were made.

If any item cannot be verified, explicitly mark it **UNVERIFIED** rather than assuming success.

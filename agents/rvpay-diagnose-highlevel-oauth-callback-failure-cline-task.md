# RVPay — Diagnostic-Only: Identify HighLevel OAuth Callback Failure

## Objective

Diagnose the current GoHighLevel OAuth installation failure in the RVPay `clients` service.

A brand-new HighLevel sub-account currently reaches the RVPay OAuth callback but HighLevel reports:

> OAuth Callback processing failed

The testing ECS task produced the following sequence for the fresh installation:

```text
id="f9d068"
{"level":"error","error":"rpc error: code = Internal desc = internal error","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/http/oauth_handler.go:94","message":"OAuth callback processing failed"}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/oauth/service.go:254","message":"\\n Location ID: t18WTYAIBHPz2uHtbcvR \\n"}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/providers/highlevel.go:153","message":"\\n HighLevelProvider ExchangeCode method initiated..."}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/oauth/service.go:235","message":"\\n State doesn't exist, resolving client/platform context... \\n "}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/oauth/service.go:202","message":"\\n HandleCallback method initiated... \\n"}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/http/oauth_handler.go:31","message":"Callback method engaged..."}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/http/oauth_handler.go:38","message":"Handler reached "}

{"level":"info","time":"2026-10-06T13:47:20Z","caller":"github.com/MountainHubTech/rvpay-go/clients/http/oauth_handler.go:53","message":"State parameter doesn't exist, given empty string value."}
```

The callback clearly reaches the application, resolves the HighLevel location ID, and enters `HighLevelProvider.ExchangeCode`. The current logs do **not** expose the underlying failure; the HTTP layer only receives `rpc error: code = Internal desc = internal error`.

This task is **diagnostic only**. Do not implement a production fix until the exact failure is established.

---

## Scope

Primary service:

```text
clients/
```

Primary code paths to inspect:

```text
clients/http/oauth_handler.go
clients/oauth/service.go
clients/providers/highlevel.go
```

Also inspect the repository/persistence/provider-setup code that is actually invoked after `ExchangeCode`, including:

- OAuth credential persistence
- integration creation/update
- client/platform resolution
- payment-provider configuration/setup
- relevant repository methods
- relevant generated SQL/sqlc code
- relevant database models/types
- migrations and constraints involved in OAuth installation

Inspect Dashboard control docs only if needed to understand an interface involved in the failure. Do not broaden the investigation unnecessarily.

---

## Required Control-Document Reading

Before diagnosing, read the relevant repository control/context files.

### Root

Read:

```text
.clinecheck
.clinerules
.clineignore
README.md
```

### Clients

Read:

```text
clients/.service-checkpoint.md
clients/.clinerules.md
clients/.clineignore.md
clients/.clinecheck.md
clients/README.md
```

If any of these files do not exist, report that fact rather than inventing a substitute.

Do not add this task file to any future Cline context/configuration.

---

## Known Context

### HighLevel OAuth configuration already established

The RVPay HighLevel integration uses the current OAuth v3 flow:

- Authorization URL:
  `https://marketplace.gohighlevel.com/oauth/chooselocation`
- Token endpoint:
  `https://services.leadconnectorhq.com/oauth/token`
- Headers:
  - `Content-Type: application/x-www-form-urlencoded`
  - `Accept: application/json`
  - `Version: v3`
- Request fields are camelCase:
  - `clientId`
  - `clientSecret`
  - `grantType`
  - `code`
  - `redirectUri`
  - `userType=Location`

Do not assume this configuration is correct merely because it was previously fixed. Verify what the current code actually does.

### Current failing location

Use this exact location ID when correlating the current incident:

```text
t18WTYAIBHPz2uHtbcvR
```

Do not log or expose the OAuth authorization code or tokens associated with this installation.

### Important state behavior

The current callback log says:

```text
State parameter doesn't exist, given empty string value.
```

The next log says:

```text
State doesn't exist, resolving client/platform context...
```

Therefore, do **not** automatically classify an empty `state` as the root cause.

Determine whether the empty-state path is an intentional/implemented fallback for HighLevel installation callbacks and whether that fallback successfully resolves the client/platform context.

---

# Diagnostic Requirements

## 1. Trace the complete callback path

Trace the exact runtime path:

```text
HTTP callback
  ↓
clients/http/oauth_handler.go
  ↓
OAuth HandleCallback
  ↓
state handling / fallback resolution
  ↓
client/platform/location resolution
  ↓
HighLevelProvider.ExchangeCode
  ↓
token response handling
  ↓
credential persistence
  ↓
integration persistence/creation
  ↓
provider configuration/setup
  ↓
callback result
  ↓
HTTP response
```

Use the actual current code rather than assuming every step exists exactly as shown.

For each boundary, identify:

- function/method
- input
- output
- error handling
- whether the error is wrapped
- whether the original error is preserved or replaced by a generic `Internal`
- repository/database operations performed

---

## 2. Identify the exact failure point

Determine which of these categories, if any, is responsible:

1. HighLevel token exchange rejected by HighLevel
2. Token exchange succeeds but response parsing fails
3. Token exchange succeeds but OAuth credential persistence fails
4. Client/platform/location resolution fails
5. Integration creation/update fails
6. Database constraint or schema error
7. Duplicate integration or credential conflict
8. Payment-provider configuration/setup fails
9. Token refresh or post-exchange provider validation fails
10. Error conversion/wrapping hides an otherwise identifiable failure
11. A regression from the recent Sub-Accounts/name work
12. Another cause not listed above

Do not choose a category based on speculation. Provide evidence.

---

## 3. Specifically investigate recent changes

Review recent changes affecting:

```text
clients/oauth/
clients/providers/highlevel.go
client/integration creation
OAuth credential persistence
database migrations
database constraints
generated SQL/sqlc
Sub-Accounts listing/name enrichment
```

Compare the relevant current code with the last known-good implementation/commit where practical.

Important:

- Do not revert changes simply because they are recent.
- Do not assume the Sub-Accounts work caused the OAuth failure.
- Determine whether there is an actual causal connection.
- If there is no evidence of a connection, explicitly say so.

If git history is available, identify the relevant commits and explain what changed along the failing path.

---

## 4. Reproduce the failure if possible

If the repository/runtime environment permits:

1. Start the relevant `clients` service.
2. Use the existing test/testing-server setup.
3. Perform a fresh HighLevel installation using a new/appropriate sub-account.
4. Correlate the callback with:
   ```text
   t18WTYAIBHPz2uHtbcvR
   ```
   or record the newly generated location ID if the test must use another location.

Capture the complete sequence from:

```text
callback received
→ state handling
→ location resolution
→ ExchangeCode
→ token response
→ persistence
→ integration creation
→ provider setup
→ final callback response
```

If reproduction is impossible, explain exactly why and continue with static/code/log diagnosis.

Do not claim runtime verification if it was not actually performed.

---

# Secret-Safe Diagnostic Logging

If existing logging is insufficient to identify the failure, you may add **temporary diagnostic logging only**.

Any temporary logging must be strictly secret-safe.

## NEVER log

Do not log:

- OAuth authorization codes
- access tokens
- refresh tokens
- client secrets
- `Authorization` headers
- cookies
- full request URLs if they contain sensitive query parameters
- complete OAuth request/response bodies
- database credentials
- API keys
- webhook secrets
- any other credential material

## Safe information that may be logged

You may log things such as:

- operation/function name
- location ID
- client/platform ID
- integration ID
- credential record ID
- whether token exchange returned successfully
- HTTP status code
- sanitized error type/message
- database operation name
- PostgreSQL constraint name
- whether an integration already exists
- whether credential persistence was attempted/succeeded
- whether provider setup was attempted/succeeded

When logging errors, preserve the original error chain.

Prefer patterns equivalent to:

```go
logger.Error().
    Err(err).
    Str("operation", "persist oauth credential").
    Str("location_id", locationID).
    Msg("OAuth installation step failed")
```

Do not construct logs that interpolate sensitive request/response contents.

---

## 5. Preserve underlying errors during diagnosis

Pay particular attention to places where code does something equivalent to:

```go
return status.Error(codes.Internal, "internal error")
```

or otherwise discards the original error.

Determine:

- where the original error first occurs;
- whether it is wrapped;
- where it becomes generic `Internal`;
- whether the existing logging already records the original error;
- whether temporary diagnostic logging is needed to expose it.

Do not redesign the service's error model as part of this task.

The goal is to **identify the existing root error**, not to perform a broad error-handling refactor.

---

## 6. Check database state

If access to the relevant testing database is available, inspect the records created/attempted for the failing installation.

Check, as applicable:

- client record
- HighLevel integration record
- OAuth credential record
- provider configuration
- external account/location ID
- unique constraints
- status fields
- timestamps
- foreign keys

Do not modify production/testing database records merely to make the installation succeed.

Read-only queries are preferred.

If a failed transaction rolls back the relevant records, document that fact.

---

## 7. Check for duplicate/conflicting state

Explicitly determine whether the new location:

```text
t18WTYAIBHPz2uHtbcvR
```

already has any:

- client
- integration
- OAuth credential
- payment-provider configuration

that could cause an insert/upsert/unique-key conflict.

Also inspect whether a partially completed prior installation could leave state that breaks reinstall.

Do not delete or mutate records as part of this diagnostic task.

---

# Strict Non-Goals

Do **NOT**:

- implement a production fix;
- change OAuth endpoints or credentials;
- change OAuth scopes;
- change the state protocol;
- change database schema;
- add migrations;
- change Sub-Accounts behavior;
- change Dashboard code;
- change Transactions;
- change webhooks;
- change CloudFormation;
- change ECS configuration;
- change ports/listeners;
- change deployment architecture;
- change payment-provider behavior;
- perform broad error-handling refactoring;
- populate missing display names;
- modify production/testing database records to force success;
- revert unrelated recent changes.

The only permitted code modification is temporary, secret-safe diagnostic logging if it is necessary to identify the existing failure.

---

# Testing Requirements

Run only tests relevant to the investigation, plus targeted regression tests needed to confirm the diagnosis.

At minimum, where applicable:

```bash
go test ./clients/...
go test ./clients/oauth/...
go test ./clients/providers/...
go vet ./clients/...
```

If these exact commands are not appropriate for the repository, use the repository's documented equivalent and explain why.

Do not add speculative tests for an unproven fix.

If tests fail because of unrelated existing issues, distinguish:

- pre-existing failure
- failure introduced by diagnostic logging
- environment/tooling failure

---

# Documentation Requirements

If diagnostic-only changes are made:

- append the investigation result to the appropriate existing `clients` checkpoint/documentation;
- do not overwrite existing checkpoint content;
- do not create unnecessary new documentation files;
- do not add this task file to project context.

The documentation should distinguish clearly between:

- diagnosed
- implemented
- executed
- verified

If no code changes are needed, say so.

---

# Temporary Logging Cleanup

If temporary diagnostic logging is added:

1. Use it to reproduce/identify the failure.
2. Capture the secret-safe evidence.
3. Remove the temporary logging before completing the task unless the logging is clearly useful, minimal, and consistent with the existing production logging policy.
4. If any temporary logging remains, explain exactly why and what it logs.

Do not leave noisy debugging logs in production code.

---

# Required Final Report

At the end, provide a concise but evidence-based report with exactly these sections:

## 1. Exact Failure / Root Cause

State the exact error if identified.

If not identified, state the precise remaining blocker.

Do not say "likely" unless the evidence genuinely cannot establish the root cause.

## 2. Evidence / Runtime Sequence

Show the important secret-safe sequence, for example:

```text
HTTP callback received
→ state fallback
→ location resolved
→ ExchangeCode started
→ token exchange succeeded/failed
→ credential persistence succeeded/failed
→ integration creation succeeded/failed
→ provider setup succeeded/failed
→ callback returned
```

Include relevant timestamps and identifiers where safe.

## 3. Exact Code Path

List the files/functions involved and where the failure originates.

## 4. Token Exchange Status

Explicitly state one of:

- token exchange succeeded
- token exchange failed
- token exchange status could not be established

Never include token/code/secret values.

## 5. Persistence Status

State what happened to:

- OAuth credential
- integration
- client
- provider configuration

## 6. Regression Assessment

State whether the failure is:

- caused by recent Sub-Accounts/name work;
- unrelated;
- or not yet established.

Support the conclusion with evidence.

## 7. Files Changed

List every modified file.

If no files were modified, say:

```text
No files modified.
```

## 8. Tests / Verification

List exact commands run and results.

Separate:

- static verification
- unit tests
- runtime verification

## 9. Temporary Diagnostic Logging

State:

- whether temporary logging was added;
- what it recorded;
- whether it was removed.

## 10. Documentation Updated

List the exact files updated, or say:

```text
No documentation updated.
```

## 11. Remaining Issues / Next Step

If the root cause is identified, recommend the **smallest possible next implementation task**.

Do not implement that fix in this task.

---

# Completion Criteria

This diagnostic task is complete only when:

- the exact failing boundary is identified, or a precise blocker is documented;
- the underlying error is captured if technically possible;
- the callback path is understood end-to-end;
- token exchange status is explicitly established or proven unobservable;
- persistence status is established;
- duplicate/constraint issues are checked;
- recent Sub-Accounts changes are assessed for causal relevance;
- no unrelated production behavior is changed;
- no secrets are exposed in logs or the final report;
- tests/verification are honestly reported;
- temporary diagnostics are cleaned up;
- the final report follows the required structure.

**Do not implement a fix merely to make the installation pass. Diagnose first.**

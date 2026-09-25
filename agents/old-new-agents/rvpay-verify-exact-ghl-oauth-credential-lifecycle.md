# Cline Task — Verify HighLevel Integration Row + OAuth Credential Lifecycle for Two Exact Locations

## Objective

Diagnose and, only where a confirmed defect is found, fix the HighLevel OAuth installation/credential lifecycle for these exact two GoHighLevel Location IDs:

- `lamyP5Es8Q2nQrb90fuv`
- `rVBsqXAKXlYLz0C96kA8`

The immediate question is:

> After reinstalling the RVPay Marketplace app with the newly requested OAuth scopes, why are the new OAuth credentials apparently not being stored/used?

This task is deliberately narrower than the previous account/customer-name work.

The goal is to establish the complete lifecycle for each location:

**GHL install/reinstall → OAuth callback → code exchange → credential persistence → integration row → credential selection at runtime → token refresh/use → location-name enrichment/persistence**

If a defect is confirmed, make the smallest targeted fix and verify it.

---

# 1. Mandatory repository context

Before changing anything, read only the relevant control/project files needed to work safely.

At minimum read:

### Root
- `.clinecheck`
- `.clinerules`
- `.clineignore`
- `README.md`

### Clients service
- `clients/.service-checkpoint.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/README.md`

### Transactions service
Only if needed to understand shared integration/token usage:
- `transactions/.service-checkpoint.md`
- `transactions/.clinerules.md`
- `transactions/.clineignore.md`
- `transactions/.clinecheck.md`
- `transactions/README.md`

Do not read broad unrelated documentation or large source trees without a concrete reason.

Append task findings to the appropriate checkpoint/documentation files. Never overwrite existing documentation.

Do not add this task file or other agent-task files to repository context/checkpoint files.

---

# 2. Exact scope

The investigation is limited to these two HighLevel Location IDs:

```text
lamyP5Es8Q2nQrb90fuv
rVBsqXAKXlYLz0C96kA8
```

For every database query and runtime verification, keep these IDs explicit.

Do not substitute arbitrary locations and do not infer that another location represents either of them.

---

# 3. Important local runtime constraint

The user has confirmed that the Transactions service must currently be manually changed to HTTP port `8081` in order to run both services locally.

This is intentional for the current local verification.

Do NOT make the Transactions port change permanent.

Do NOT modify CloudFormation, ECS listeners, load balancers, production ports, or deployment infrastructure as part of this task.

Do NOT spend time solving the port architecture.

For local testing only, use:

- Clients service: existing local port/configuration
- Transactions service: manually run on `8081` if both are needed

The existing production/deployment port arrangement is out of scope.

---

# 4. Primary investigation questions

Answer these questions with concrete evidence.

## A. Integration row

For each exact location ID:

1. Does an integration row exist?
2. What provider is recorded?
3. What `external_account_id` is recorded?
4. What `client_name` is recorded?
5. What `display_name` is recorded?
6. What status/active fields exist and what are their values?
7. What timestamps identify creation/update?
8. Are there duplicate integration/client rows for the same location?
9. If duplicates exist, which row is actually selected by application code?

Do not expose secrets.

Record only safe metadata.

---

## B. OAuth credential lifecycle

Trace the actual code path from:

```text
GHL Marketplace install/reinstall
        ↓
OAuth callback
        ↓
authorization code
        ↓
ExchangeCode
        ↓
credential/token persistence
        ↓
integration/client association
        ↓
runtime credential lookup
        ↓
HighLevel API call
```

Identify the exact functions/files involved.

For each exact location ID, determine:

1. Was an OAuth callback actually received?
2. Was the authorization code exchanged successfully?
3. Was the token exchange response successfully parsed?
4. Was a credential/token record inserted or updated?
5. Which database table stores it?
6. Which row/record corresponds to the location?
7. Was an existing credential updated/replaced, or was a new credential inserted?
8. Is there any uniqueness/upsert conflict that prevents the new credential from being stored?
9. Is the integration row linked to the credential correctly?
10. Does runtime lookup select the newly stored credential?
11. Is runtime lookup accidentally selecting an older credential?
12. Is the credential lookup keyed by location ID, integration ID, client name, provider, or another field?
13. Is there any transaction/rollback path that causes the credential write to disappear after a successful token exchange?

The final report must distinguish:

- **not received**
- **received but exchange failed**
- **exchange succeeded but persistence failed**
- **persistence succeeded but wrong row selected**
- **correct credential stored and selected**
- **unknown because runtime evidence is insufficient**

Do not collapse these into "OAuth is broken."

---

# 5. Safe credential verification

Never print:

- access tokens
- refresh tokens
- client secrets
- authorization codes
- Authorization headers
- cookies
- full credential blobs

Do not add logging that exposes secrets.

If the schema/code permits safe verification, use metadata such as:

- credential/integration row ID
- created_at
- updated_at
- expires_at
- token type
- provider
- external account/location ID
- encrypted/token-present boolean
- non-reversible fingerprint/hash of a token, if an existing safe mechanism exists

If a fingerprint/hash is introduced temporarily for diagnosis, it must never log the raw token and must be removed after verification unless the repository already has an established safe credential-fingerprint mechanism.

The report should be able to say:

```text
location X:
- integration row exists
- credential row exists
- credential updated at <timestamp>
- credential token fields are populated
- runtime lookup selected credential row <id>
- raw token not displayed
```

That is sufficient.

---

# 6. Determine whether the "new credentials" are actually reaching RVPay

The user manually reinstalled the Marketplace app after adding the required OAuth scope(s).

Investigate whether the reinstall actually causes a new OAuth installation flow for:

- `lamyP5Es8Q2nQrb90fuv`
- `rVBsqXAKXlYLz0C96kA8`

Specifically inspect:

- install URL generation
- redirect URI
- OAuth state
- callback handler
- authorization code exchange
- `user_type=Location`
- OAuth v3 headers
- OAuth v3 camelCase request fields
- token response parsing
- integration creation/update
- credential creation/update

Confirm whether the reinstall overwrites an existing credential or creates another one.

Do not assume "I reinstalled it" means the backend received a new token.

Prove it from runtime/database evidence where possible.

---

# 7. Check the exact current OAuth implementation

Inspect the current implementation of:

- HighLevel provider OAuth configuration
- `ExchangeCode`
- `RefreshToken`
- OAuth callback handler
- OAuth service
- integration repository/service
- credential repository/service
- any install webhook handler that creates/updates integrations
- any runtime credential resolver/provider registry

Confirm that the current implementation consistently uses the HighLevel v3 contract already established by the project:

### Authorization

```text
https://marketplace.gohighlevel.com/oauth/chooselocation
```

### Token endpoint

```text
https://services.leadconnectorhq.com/oauth/token
```

### Headers

```text
Content-Type: application/x-www-form-urlencoded
Accept: application/json
Version: v3
```

### Request fields

```text
clientId
clientSecret
grantType
code
redirectUri
userType=Location
```

Do not change these merely because they are being inspected.

Only change code if the current implementation is demonstrably inconsistent with the actual runtime failure.

---

# 8. Database investigation

Use the actual local Clients database/environment currently used by the Clients service.

Do not merely inspect migration files.

Verify the live schema and live rows.

For both exact locations, inspect:

- clients/integration records
- credential/token records
- provider identifiers
- external account/location IDs
- timestamps
- uniqueness constraints
- foreign keys
- status fields
- display name

Also inspect whether the migration that added `clients.display_name` is actually applied.

Do not run destructive migrations.

Do not delete or rewrite production data.

If a backfill/reconciliation command already exists, identify it, but do not execute it until the credential lifecycle is understood.

---

# 9. Runtime verification

Start the local services needed for this investigation.

The user has confirmed that both services can be run locally only if Transactions is manually changed to port `8081`.

Respect that existing local workaround.

Do not make the port change permanent.

Verify the Clients service is actually using the same database you inspect.

Verify the Transactions service is actually using its intended local database if it is needed.

Do not assume localhost/database names based solely on source code; inspect the actual runtime configuration.

---

# 10. Reproduce the relevant flow

Where practical, reproduce the actual OAuth/install flow using the existing project mechanisms.

If an actual browser-based GHL reinstall cannot be safely automated, do not fake one.

Instead:

1. inspect logs/database state;
2. identify the exact callback/install events that occurred;
3. correlate them to the two location IDs;
4. determine whether the backend received a fresh authorization code;
5. determine whether the code exchange succeeded;
6. determine whether persistence occurred.

If live GHL access is unavailable, state exactly which lifecycle step cannot be verified and why.

Do not claim that a new credential was stored merely because an install UI was used.

---

# 11. Important distinction: integration vs credential

Treat these as separate objects.

Do not conclude:

> "The integration exists, therefore the new OAuth credential exists."

Instead prove both independently:

```text
Location
  ↓
Integration row
  ↓
Credential association
  ↓
Credential row/version
  ↓
Runtime credential selection
```

If the project has a provider abstraction, trace how it resolves the credential at runtime.

---

# 12. Investigate the `lamyP5Es8Q2nQrb90fuv` discrepancy

Previous verification found:

```text
rVBsqXAKXlYLz0C96kA8
```

present in the active Clients database.

Previous verification found:

```text
lamyP5Es8Q2nQrb90fuv
```

absent from the active Clients database.

This discrepancy is now a primary investigation target.

Determine whether:

- the reinstall happened against another local database;
- the callback was never received;
- the integration was rejected/rolled back;
- the location was stored under a different external account ID;
- the install path uses another environment;
- an existing row was deleted/replaced;
- the callback reached a different deployed instance;
- the location is present under another provider/integration record;
- the database being inspected is not the one used by the running service.

Do not speculate. Trace the evidence.

---

# 13. Check logging around the OAuth lifecycle

Inspect existing logs for the two exact locations.

Search for:

- location IDs
- integration IDs
- OAuth callback events
- token exchange success/failure
- install events
- credential persistence
- database errors
- unique constraint errors
- transaction rollback errors

If current logging is insufficient, add only narrowly scoped, secret-safe diagnostic logging needed to prove the lifecycle.

Do not log raw credentials.

If diagnostic logging is added, document it and remove it after verification unless it is clearly appropriate as permanent safe operational logging.

---

# 14. Only fix confirmed defects

If the investigation finds a concrete defect, make the smallest fix necessary.

Examples of acceptable fixes, only if actually proven:

- credential upsert fails due to incorrect key;
- integration is not linked to the newly persisted credential;
- successful token exchange never reaches credential persistence;
- persistence occurs in a transaction that is later rolled back;
- reinstall incorrectly reuses stale credential;
- runtime resolver selects an older credential;
- location ID is not propagated into the credential/integration association;
- token response fields are parsed incorrectly;
- v3 request is malformed.

Do NOT:

- redesign the OAuth architecture;
- add a new credential system;
- change CloudFormation;
- change ECS listeners;
- change production ports;
- modify unrelated HighLevel APIs;
- modify native order synchronization;
- modify inbound webhook delivery;
- modify Dashboard rendering unless this investigation proves the backend contract is correct but the Dashboard is independently wrong.

---

# 15. Account-name enrichment

Only after credential lifecycle is understood, verify whether the stored/selected credential can successfully call the HighLevel Location API for the exact location.

For each location where a valid credential is proven:

1. perform the existing location-name lookup mechanism;
2. verify the HighLevel response;
3. verify the actual name extracted;
4. verify `clients.display_name` is updated;
5. verify subsequent Sub-Accounts API response contains that name.

Do not treat failure here as proof that OAuth persistence is broken.

Keep these stages separate:

```text
credential persistence
→ credential selection
→ HighLevel API authorization
→ location lookup
→ display_name persistence
→ Dashboard rendering
```

---

# 16. Tests

Add or update focused tests only for confirmed defects.

At minimum, where applicable, test:

- new credential insert/upsert;
- reinstall replacing/updating existing credential;
- integration-to-credential association;
- runtime credential selection;
- location-specific credential lookup;
- duplicate/uniqueness behavior;
- rollback behavior if relevant.

Run the narrowest relevant tests first.

Then run broader service tests if practical.

Do not claim tests passed unless actually executed.

---

# 17. Documentation

Append the investigation outcome to the appropriate existing checkpoint/documentation files.

Include:

- exact locations investigated;
- integration row findings;
- credential lifecycle findings;
- whether reinstall generated a new callback;
- whether token exchange succeeded;
- whether credential persistence succeeded;
- which credential row runtime selects;
- whether location-name lookup succeeded;
- any confirmed defect and fix;
- tests actually run;
- runtime verification actually performed;
- remaining limitations.

Do not overwrite existing checkpoints.

Do not add speculative conclusions.

---

# 18. Final report — mandatory format

End with a concise but evidence-based report containing exactly these sections:

## 1. Executive finding

State in plain language what happened to the OAuth credentials for:

- `lamyP5Es8Q2nQrb90fuv`
- `rVBsqXAKXlYLz0C96kA8`

## 2. Integration rows

Table:

| Location ID | Integration exists | Provider | external_account_id | client_name | display_name | Status | Integration timestamps | Duplicate rows |
|---|---|---|---|---|---|---|---|---|

## 3. Credential lifecycle

Table:

| Location ID | Callback received | Code exchange | Credential persisted | Credential updated/replaced | Runtime-selected credential | Token metadata | Result |
|---|---|---|---|---|---|---|---|

Never include raw tokens.

## 4. Root cause

State the exact confirmed root cause(s), or explicitly state which lifecycle step could not be proven.

## 5. Changes made

List every modified file and why.

If no code was changed, say so.

## 6. Verification

List exact commands/tests/runtime checks performed and their results.

Clearly distinguish:

- implemented
- executed
- verified

## 7. Remaining uncertainty

Only list genuinely unverified items.

## 8. Next action

Give the smallest concrete next action required, if any.

---

# Definition of done

This task is complete only when the report can answer, with evidence, for BOTH:

```text
lamyP5Es8Q2nQrb90fuv
rVBsqXAKXlYLz0C96kA8
```

1. Does the integration row exist?
2. Does the credential row exist?
3. When was the credential last created/updated?
4. Did the reinstall produce a new OAuth callback?
5. Did the authorization code exchange succeed?
6. Was the resulting credential persisted?
7. Was it associated with the correct location/integration?
8. Which credential does runtime actually select?
9. Can that credential authorize the HighLevel Location API call?
10. If authorized, does the location name get persisted to `clients.display_name`?
11. If anything failed, exactly where in the lifecycle did it fail?

If an answer cannot be proven, explicitly mark it **UNVERIFIED** and explain why.

Do not fill gaps with assumptions.

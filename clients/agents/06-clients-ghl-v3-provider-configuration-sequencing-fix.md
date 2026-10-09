# Agent 06 — GHL v3 Provider Configuration Sequencing Fix

## Mission

Perform a **surgical Clients-only correction** to the existing HighLevel v3 Custom Payment Provider registration flow.

This is **Agent 06 — Final Review**.

The Clients service has already been implemented and reviewed by previous agents. **Do not rebuild, redesign, or rediscover the service.** Read the existing service documentation and checkpoint first, understand what is already correct, and change only what is necessary to fix the currently observed HighLevel behavior.

### REQUIRED FILES TO READ FIRST

Before inspecting or modifying implementation code, read these files in full:

1. `clients/.clineignore.md`
2. `clients/.clinecheck.md`
3. `clients/.clinerules.md`
4. `clients/.service-checkpoint.md`
5. `clients/README.md`

These files are the authoritative context for this task.

Use:

* `clients/.service-checkpoint.md` to understand the service architecture, previous agents' work, decisions, boundaries, and remaining issues.
* `clients/README.md` to understand the service's intended behavior and architecture.
* `clients/.clinecheck.md` to understand the current implementation/check status.
* `clients/.clinerules.md` and `clients/.clineignore.md` as strict operating constraints.

**STRICTLY ADHERE to `clients/.clinerules.md` and `clients/.clineignore.md`.**

Do not assume the repository has the same state as a previous Cline session. The implementation has changed since earlier work.

---

# Current Problem

A fresh HighLevel Marketplace installation successfully reaches the RVPay Clients service.

The OAuth installation succeeds.

The correct `locationId` is obtained.

The HighLevel provider association is attempted.

However, HighLevel currently returns:

```text
Base config for integration is not created yet.
Please create the base config before updating integration keys.
```

Specifically:

```text
HTTP 422
```

from the provider configuration operation.

The current implementation performs the equivalent of:

```text
POST /payments/custom-provider/provider
        ↓
POST /payments/custom-provider/connect
```

The required sequence is:

```text
POST /payments/custom-provider/provider
        ↓
GET /payments/custom-provider/connect
        ↓
confirm base config exists
        ↓
POST /payments/custom-provider/connect with credentials
```

Fix **only this lifecycle/sequencing issue**.

---

# Hard Boundaries

Modify only what is necessary in:

```text
clients/
```

The explicitly permitted documentation/checkpoint updates are:

```text
clients/.clinecheck.md
clients/.service-checkpoint.md
clients/README.md
```

Do NOT modify:

* `admindashboard/`
* `transactions/`
* `deposits/`
* protobuf files
* generated protobuf files
* legacy `integrations/`
* unrelated root infrastructure
* deployment configuration
* Marketplace dashboard configuration

Do not redesign the Clients service.

Do not rewrite working OAuth or Marketplace callback behavior.

Do not rewrite client provisioning.

Do not rewrite integration provisioning.

Do not alter database structure unless the existing implementation proves it is absolutely necessary for this specific fix.

Prefer **zero DB changes**.

---

# Preserve Existing Agent Work

The following existing behavior must remain intact unless the current code demonstrably violates the documented HighLevel v3 contract:

* stateless Marketplace callback handling
* exactly one OAuth authorization-code exchange
* correct OAuth v3 token exchange
* `locationId` obtained from the token response
* HighLevel platform lookup by slug `highlevel`
* idempotent `highlevel-<locationId>` client
* idempotent integration using `external_account_id = locationId`
* `processCallbackWithToken(...)` convergence
* existing OAuth token persistence
* existing provider abstraction
* existing `PaymentProviderClient` boundary
* existing `RegisterProvider(...)` orchestration
* existing provider metadata configuration
* existing environment configuration
* existing live/test credential handling
* existing local provider configuration persistence
* existing idempotency behavior
* existing typed errors
* existing safe logging conventions

Do not make unrelated refactors.

---

# Verify the Existing HighLevel Client

Inspect the current implementation of the existing payment-provider client, particularly:

* `CreateProviderAssociation()`
* `FetchProviderConfig()`
* `CreateProviderConfigs()`
* `PaymentProviderClient`
* the HTTP request/response handling beneath those methods
* `RegisterProvider()`
* `CreateProviderConfigs()`

Do not create duplicate methods if the required capability already exists.

Use the existing abstraction.

---

# HighLevel v3 Contract

Verify the implementation against the official HighLevel v3 payment documentation already identified by the previous agents:

* Custom Payment Provider
* Create Integration
* Create Config
* Fetch Config

The relevant operations are:

```text
POST /payments/custom-provider/provider
GET  /payments/custom-provider/connect
POST /payments/custom-provider/connect
```

The implementation must continue using:

```text
Version: v3
Authorization: Bearer <access token>
```

with the correct `locationId` query parameter.

Do not substitute older HighLevel API contracts.

---

# Required Runtime Sequence

Change the existing orchestration so that it behaves as follows:

```text
1. POST /payments/custom-provider/provider
        │
        ▼
2. GET /payments/custom-provider/connect?locationId=<locationId>
        │
        ├── base configuration exists
        │       │
        │       ▼
        │   3. POST /payments/custom-provider/connect
        │      with live/test credentials
        │
        └── base configuration does not yet exist
                │
                ▼
            do NOT POST credentials yet
```

The critical requirement is:

**Never send the credential/configuration POST until the base configuration has been confirmed to exist.**

---

# Eventual Consistency

If HighLevel creates the base configuration asynchronously after `/provider` succeeds, use a **small bounded verification retry** around:

```text
GET /payments/custom-provider/connect
```

only.

Do NOT:

* sleep arbitrarily and assume success
* repeatedly POST `/provider`
* repeatedly POST credentials
* blindly retry the 422 credential operation

If retrying is necessary:

```text
POST /provider
        ↓
GET /connect
        ↓
not ready
        ↓
GET /connect
        ↓
not ready
        ↓
bounded failure
```

Once the GET confirms the base config exists:

```text
POST /connect credentials
```

must happen.

Use existing retry/error conventions in `clients/` if available.

Do not introduce a broad retry framework for this one operation.

---

# Error Semantics

Do not classify every `400` or `422` as "already exists."

Distinguish appropriately between:

* existing association
* existing base configuration
* base configuration not yet available
* invalid request
* unauthorized/expired token
* insufficient permission
* unexpected HighLevel error

If the existing response handling cannot distinguish these states, make the smallest necessary change at the existing provider-client abstraction boundary.

Do not weaken error handling simply to make the installation appear successful.

---

# Provider Credentials

Continue using the existing environment-driven configuration.

Do not introduce a new configuration system.

Preserve:

* live API key
* live publishable key
* test API key
* test publishable key

Do not alter credential semantics.

Never log:

* OAuth access tokens
* refresh tokens
* client secrets
* API keys
* publishable keys
* authorization codes

If the currently touched code logs sensitive credentials, remove/redact those logs as part of this fix.

---

# Idempotency

Preserve the existing idempotent behavior.

An already registered HighLevel provider must remain safe to process again.

An existing local provider configuration must continue to reuse its valid local API key rather than unnecessarily generating another one.

Remote metadata must not be replaced with empty/default values merely because a fetch fails.

Do not change local persistence behavior unless required by this sequencing fix.

---

# Tests

Add or update **focused tests only**.

Tests must verify the actual ordering.

At minimum cover:

1. `/provider` occurs first.
2. GET `/connect` occurs after `/provider`.
3. credential POST does not occur before base-config confirmation.
4. credential POST occurs after successful base-config confirmation.
5. eventual-consistency verification retry, if implemented.
6. exhausted verification does not send credentials.
7. HighLevel 422 is not automatically treated as "already exists."
8. unauthorized responses remain unauthorized.
9. unexpected responses remain errors.
10. existing/idempotent provider registration continues to work.
11. existing OAuth/callback behavior remains unaffected.

Use the existing Clients testing conventions.

Do not rewrite unrelated tests.

---

# Verification

Run:

```text
go test ./clients/... -count=1
go vet ./clients/...
```

Run `gofmt` on changed hand-written Go files.

Do not fix unrelated failures.

If a check cannot run because of the environment, report that honestly.

Do not claim a live HighLevel test unless one was actually performed.

---

# Documentation / Completion Files

## `.clinecheck.md`

UPDATE:

```text
clients/.clinecheck.md
```

with the final implementation status, affected code, sequencing behavior, and verification results.

Do not destroy useful existing information.

Preserve the existing structure where possible.

## `.service-checkpoint.md`

**APPEND ONLY** to:

```text
clients/.service-checkpoint.md
```

Do NOT overwrite previous agent reports.

Add:

```text
Agent 06 — GHL v3 Provider Configuration Sequencing Fix
```

Include:

* observed HighLevel 422
* root cause
* previous sequence
* corrected sequence
* exact implementation change
* whether bounded verification retry was required
* files changed
* tests/checks run
* results
* remaining limitations
* confirmation that unrelated Clients functionality was preserved

## `README.md`

Update:

```text
clients/README.md
```

only if necessary to accurately document the corrected provider-registration lifecycle.

Do not rewrite the README.

Keep the change minimal and consistent with its existing documentation.

---

# Final Constraints

This is a **FINAL REVIEW / SURGICAL FIX**, not a rewrite.

Before changing anything, understand the existing Clients service through:

```text
clients/.service-checkpoint.md
clients/README.md
clients/.clinecheck.md
clients/.clinerules.md
clients/.clineignore.md
```

Then inspect only the implementation necessary for the GHL provider configuration lifecycle.

If existing code is already correct, **leave it alone**.

Do not:

* refactor unrelated code
* rename unrelated functions
* reorganize packages
* redesign interfaces unnecessarily
* modify frontend code
* modify Transactions
* modify protobufs
* modify deployment
* modify OAuth unless the provider-registration fix absolutely requires it

The single functional objective is:

```text
POST /payments/custom-provider/provider
        ↓
GET /payments/custom-provider/connect
        ↓
confirm base config exists
        ↓
POST /payments/custom-provider/connect with credentials
```

while preserving everything from the previous Clients agents that already works.

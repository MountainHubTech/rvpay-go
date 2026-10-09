# Agent 02 — HighLevel Marketplace OAuth Fix

## Objective
Implement only the HighLevel Marketplace INSTALL/OAuth changes identified by Agent 01.
Use `clients/.service-checkpoint.md` and `clients/.clinecheck.md` as the primary context.

## Required behavior

### 1. INSTALL establishes RVPay tenancy
Use the existing HighLevel INSTALL webhook flow.

For a valid HighLevel `locationId`:
1. Resolve/reuse the existing client for that HighLevel sub-account.
2. Create the client when it does not exist.
3. Resolve/reuse the existing integration for that client and the existing HighLevel platform.
4. Create the integration when it does not exist.
5. Store the HighLevel `locationId` in the existing integration mapping field (`external_account_id`) according to the current schema/repository conventions.
6. Make the operation idempotent.

The HighLevel platform already exists.
Do not create or modify platform records.

Do not invent new database structures if existing columns, queries, repositories, or patterns can support this.

### 2. OAuth callback resolves the pre-established integration
The Marketplace callback may not contain OAuth `state`.
After exchanging the authorization code, use the returned HighLevel `locationId` to resolve the integration established by INSTALL.
Do not require an OAuth state record for this Marketplace flow.

### 3. Exchange the authorization code exactly once
The callback flow must exchange the authorization code once.
Do not pass the raw authorization code into a function that exchanges it again.
Pass the already-exchanged token response into downstream processing, using the project's existing types/patterns or the smallest necessary change.

### 4. Preserve existing state-based OAuth flow
Do not break the existing `state != ""` flow.
Only adjust shared code where required so both flows remain correct.

### 5. Tests
Update/add focused unit/integration tests for:
- INSTALL creates client + integration.
- Repeated INSTALL reuses existing records.
- Integration maps to HighLevel `locationId`.
- Marketplace callback resolves the integration by `locationId`.
- Authorization code is exchanged exactly once.
- Existing state-based callback behavior remains intact.
- Relevant repository/SQL behavior is covered using existing test conventions.

Run the narrowest relevant tests first, then the clients-service test suite.
Do not spend tokens running unrelated repository-wide work unless required to diagnose a directly affected failure.

## Strict scope
Allowed changes:
- `clients/` files directly involved in the INSTALL/OAuth call path.
- Their directly affected repositories/SQL queries/migrations only when required by the existing schema design.
- Directly affected tests.
- `clients/README.md` only if behavior/documentation needs updating.

Forbidden:
- Transactions service.
- Deposits service changes.
- PawaPay changes.
- Platform creation/modification.
- Infrastructure/CloudFormation changes.
- Secrets changes.
- Unrelated refactoring.
- Generated/dependency files unless regeneration is required by a direct source change.
- New abstractions merely for style.

## Context management
Read only files identified by Agent 01 and files directly required by their call graph.
Do not recursively inspect unrelated directories.
Use `.clineignore.md` and `.service-checkpoint.md` as the context boundary.
Keep reasoning and notes short.
After each implementation task, update `clients/.clinecheck.md` and `clients/.service-checkpoint.md`.

## Final verification
Before finishing:
1. Run focused changed-package tests.
2. Run the clients-service tests.
3. Inspect the diff for unrelated changes.
4. Update `.clinecheck.md` with completed/remaining items.
5. Update `.service-checkpoint.md` with implementation, tests, and any remaining risk.

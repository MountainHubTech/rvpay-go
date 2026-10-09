# Agent 01 — Clients OAuth Context Sweep

## Objective
Perform a focused reconnaissance of `clients/` for the HighLevel Marketplace OAuth installation fix.
Do not modify application code.

## Scope
Investigate only files that can affect:
- HighLevel INSTALL webhook handling.
- OAuth callback handling.
- Client creation/retrieval.
- Integration creation/retrieval and `external_account_id`.
- HighLevel platform lookup (platform already exists; do not create it).
- OAuth token/code exchange.
- Related repositories, SQL queries, migrations, dependency wiring, configuration/envs, HTTP routes, and tests.

Read the clients service README and relevant package documentation first.
Use the existing legacy deposits service only as a reference where it explains established RVPay patterns.

## Context-depth rules
Start from `clients/cmd/grpc-service/main.go` and the relevant OAuth/webhook entry points.
Follow imports only as needed to establish the call path.
Do not recursively read every package.
Avoid generated files, vendor/dependency trees, build output, fixtures, unrelated services, and heavy directories unless a specific dependency is required to understand this change.
Use `clients/.clineignore.md` after creating it; keep it narrow and explicit.

## Database investigation
Identify the SQL schema, migrations, sqlc queries, repositories, and models for:
- `clients`
- `integrations`
- `platforms`
- OAuth state/tokens if relevant
- HighLevel payment-provider configuration if relevant

Inspect the actual SQL queries used for lookup/create/update operations.
Determine the current constraints/indexes that affect idempotent creation and `external_account_id` mapping.
Do not change migrations or schema.

## Required deliverables
Create these files inside `clients/`:
- `.clineignore.md`
- `.clinerules.md`
- `.clinecheck.md`
- `.service-checkpoint.md`

`.clinerules.md` must enforce:
- Only this OAuth/INSTALL change.
- No unrelated refactors.
- No new architectural patterns when an existing RVPay pattern exists.
- Tests are mandatory for changed behavior.
- Minimal file reading and minimal tool/token use.
- Preserve existing syntax/naming/error/repository conventions.
- Platform creation is explicitly out of scope.
- Do not alter secrets, deployment infrastructure, or unrelated services.

`.clinecheck.md` must contain a concise checkbox task plan for Agent 02.
`.service-checkpoint.md` must record the discovered architecture, exact files/packages relevant to Agent 02, DB query/schema findings, risks, and explicit files/folders to avoid.
Keep both documents concise enough to serve as the primary context source for Agent 02.

## Completion
Verify the four files exist and are internally consistent.
Do not modify application code.

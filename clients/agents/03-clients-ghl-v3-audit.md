# Agent 03 — Clients GHL v3 Audit

## Mission
Audit ONLY `clients/`. Do not implement fixes. Produce a precise implementation map for Agent 04.

Use **HighLevel API / Marketplace v3 only**. Never use legacy/older GHL docs or endpoints.

Official v3 references:
- https://marketplace.gohighlevel.com/docs/ghl/oauth/oauth-2-0-v-3/
- https://marketplace.gohighlevel.com/docs/ghl/oauth/get-access-token/
- https://marketplace.gohighlevel.com/docs/ghl/payments/custom-provider/index.html
- https://marketplace.gohighlevel.com/docs/ghl/payments/create-integration/
- https://marketplace.gohighlevel.com/docs/ghl/payments/create-config/
- https://marketplace.gohighlevel.com/docs/ghl/payments/fetch-config/
- https://marketplace.gohighlevel.com/docs/marketplace-modules/Payments/

## Scope
Read and reason about:
- `clients/oauth/`
- `clients/providers/`
- `clients/integrations/`
- `clients/clients/`
- `clients/platforms/`
- relevant `clients/db/query/`
- relevant `clients/db/migrations/`
- relevant `clients/db/repo/`
- relevant tests
- `clients/config/`
- `clients/cmd/grpc-service/main.go`
- root `README.md`
- root `.clinerules.md` / `.clineignore`
- `agents/project-context.md`
- `clients/README.md`
- `clients/.clinerules.md` / `clients/.clineignore` if present

Avoid:
- `transactions/`
- `deposits/`
- legacy `integrations/`
- unrelated root infrastructure
- unrelated Clients packages
- generated code except to understand dependencies

## Read-depth classification
For each relevant Clients directory classify it:
- `READ-DEEP`: directly on install → OAuth → provider registration path
- `READ-SHALLOW`: interfaces/models/config needed for context
- `AVOID`: unrelated
- `GENERATED`: never hand-edit

Map this path:
`GHL callback → code exchange → locationId → client → integration → token persistence → provider registration → HighLevel API → local provider config`

## Database
Inspect Clients SQL/migrations/schema and, if a development DB is available, use READ-ONLY SQL to verify:
- clients table and uniqueness
- integrations table and uniqueness
- provider configuration table
- foreign keys
- status fields
- location/external account fields
- provider metadata columns
- existing affected rows

Never mutate the DB in this task.

## Critical v3 audit
Determine from v3 docs, with exact endpoint/method/payload:
1. Provider association endpoint.
2. Whether `locationId` is query parameter or body.
3. Required provider metadata fields.
4. Provider configuration/connect endpoint.
5. Fetch configuration endpoint.
6. Correct token/user type.
7. Required payment scopes.
8. Agency/company vs location installation behavior.
9. Correct handling of existing association/configuration.
10. Which remote response actually represents the provider metadata that should appear in HighLevel.

Pay special attention to the distinction between:
- creating the app/location Custom Payment Provider association, and
- creating/fetching provider connection/configuration.

Compare this against the current `HighLevelPaymentProviderClient` implementation. Do not fix it here.

## Required files
Create/update inside `clients/`:

### `.clineignore.md`
Keep context narrow. Exclude unrelated services and unrelated Clients code, while allowing all files needed for OAuth, providers, integrations, clients, platforms, DB, tests, and service wiring.

### `.clinerules.md`
State:
- Clients only.
- GHL v3 only.
- Build upon existing edits; no wholesale rewrite.
- Preserve single OAuth-code exchange.
- Preserve current Marketplace stateless callback design unless a proven defect requires change.
- Platform already exists; never create it.
- Minimal changes only.
- Never log credentials/codes.
- Never hand-edit generated code.
- Inspect DB/schema before repository changes.
- Update tests with code changes.
- Do not touch unrelated services.
- Record Marketplace dashboard prerequisites at the end.

### `.clinecheck.md`
Include:
- files inspected
- dependency/read-depth map
- DB findings
- v3 endpoint findings
- current defects
- exact files Agent 04 should modify
- exact files/directories to avoid
- required tests
- Marketplace dashboard items
- risks/open questions

### `.service-checkpoint.md`
Create a baseline section:
`Agent 03 — Audit Baseline`
Do not overwrite past or future entries.

## Completion
Do not implement fixes.
Update `.clinecheck.md` and append `.service-checkpoint.md`.

# Agent 05 — Clients GHL v3 Final Verification

## Mission
Perform a focused final review of the Clients HighLevel Marketplace/payment-provider work.

Do not redesign or perform unrelated cleanup.

Use **HighLevel v3 only**:
- https://marketplace.gohighlevel.com/docs/ghl/oauth/oauth-2-0-v-3/
- https://marketplace.gohighlevel.com/docs/ghl/oauth/get-access-token/
- https://marketplace.gohighlevel.com/docs/ghl/payments/custom-provider/index.html
- https://marketplace.gohighlevel.com/docs/ghl/payments/create-integration/
- https://marketplace.gohighlevel.com/docs/ghl/payments/create-config/
- https://marketplace.gohighlevel.com/docs/ghl/payments/fetch-config/
- https://marketplace.gohighlevel.com/docs/marketplace-modules/Payments/

## Read first
Read:
- root README/rules/ignore
- `agents/project-context.md`
- `clients/.clinerules.md`
- `clients/.clineignore.md`
- `clients/.clinecheck.md`
- `clients/.service-checkpoint.md`
- all files changed by Agent 04
- relevant Clients DB schema/query/repository/migrations
- relevant tests

Do not inspect unrelated services recursively.

## Verify installation flow
Confirm:
`GHL install → callback → one code exchange → locationId → client → integration → token → v3 provider registration → local provider persistence`

Verify:
- no second OAuth code exchange
- correct location ID, not installing user ID
- existing HighLevel platform is looked up, not created
- client creation/reuse is idempotent
- integration creation/reuse is idempotent
- token persistence remains intact

## Verify GHL v3 contract
Check actual code against v3 docs:
- method
- path
- `Version: v3`
- query params
- request body
- bearer auth
- response parsing
- 400/401/422 behavior

Pay special attention to the difference between:
- Custom Provider association
- Custom Provider connect/configuration

A row in RVPay's DB is NOT proof that HighLevel registered the provider correctly.

## DB verification
If a development DB is available, use read-only SQL to verify:
- client for location
- integration for client/platform
- `external_account_id = locationId`
- integration status
- provider config row
- provider metadata fields
- correct integration association

Do not mutate DB state.

## Tests/checks
Run:
- `go test ./clients/... -count=1`
- `go vet ./clients/...`
- formatting checks on changed hand-written Go

Verify generated code is consistent where relevant. Never hand-edit generated files.

## Scope review
Flag/revert only clearly unrelated changes:
- other services
- legacy API usage
- broad OAuth rewrite
- unrelated refactors
- credential logging
- unnecessary schema changes

Do not rewrite working code for style alone.

## Final readiness checklist
- [ ] Marketplace callback reaches Clients.
- [ ] Single OAuth exchange.
- [ ] Correct location ID.
- [ ] Existing platform resolved.
- [ ] Client idempotent.
- [ ] Integration idempotent.
- [ ] Token persisted.
- [ ] Provider registration uses v3.
- [ ] Association targets location.
- [ ] Provider metadata is sent via correct v3 contract.
- [ ] Payment/query URLs are correct public RVPay endpoints.
- [ ] Existing provider/config handled safely.
- [ ] Local provider config is correct.
- [ ] Errors are meaningful.
- [ ] Credentials are not logged.
- [ ] Tests pass.
- [ ] No unrelated service changed.

## Marketplace dashboard report
Append to `.service-checkpoint.md`:

`Agent 05 — Marketplace Dashboard — Final Verification`

Separate:
1. Required
2. Recommended
3. Not required / API-handled

Verify v3 requirements for:
- payment scopes
- OAuth redirect
- Custom Payment Provider capability/module
- webhook settings
- public HTTPS payment/query endpoints

## Completion
Update `.clinecheck.md` without deleting historical findings.

APPEND to `.service-checkpoint.md`:
`Agent 05 — Final Verification`

Include:
- verification performed
- tests
- DB verification
- v3 contract verification
- dashboard requirements
- remaining risks
- exact manual next steps, if any

Return a concise PASS/FAIL report.

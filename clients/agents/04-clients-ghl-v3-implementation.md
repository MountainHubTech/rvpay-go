# Agent 04 — Clients GHL v3 Implementation

## Mission
Implement the HighLevel Marketplace installation → automatic Custom Payment Provider registration flow in `clients/`.

Start with Agent 03's audit. Do not rediscover the whole repository.

Use **HighLevel API / Marketplace v3 only**:
- https://marketplace.gohighlevel.com/docs/ghl/oauth/oauth-2-0-v-3/
- https://marketplace.gohighlevel.com/docs/ghl/oauth/get-access-token/
- https://marketplace.gohighlevel.com/docs/ghl/payments/custom-provider/index.html
- https://marketplace.gohighlevel.com/docs/ghl/payments/create-integration/
- https://marketplace.gohighlevel.com/docs/ghl/payments/create-config/
- https://marketplace.gohighlevel.com/docs/ghl/payments/fetch-config/
- https://marketplace.gohighlevel.com/docs/marketplace-modules/Payments/

## Hard boundaries
Modify only what is necessary under `clients/`.

Do NOT modify:
- `transactions/`
- `deposits/`
- legacy `integrations/`
- unrelated root infrastructure
- Marketplace dashboard itself
- generated files by hand

Do not redesign the Clients service.

## Preserve existing work
Build upon, do not rewrite:
- stateless Marketplace callback support
- exactly one OAuth authorization-code exchange
- `locationId` from the token response
- existing HighLevel platform lookup by slug `highlevel`
- idempotent client `highlevel-<locationId>`
- idempotent integration with `external_account_id = locationId`
- `processCallbackWithToken(...)` convergence
- existing token persistence
- existing credential-safe logging

Change only where v3 proves the current behavior is wrong.

## Required end state
For a sub-account installation, including an agency/company-initiated installation targeting a sub-account:
1. Exchange code once.
2. Resolve correct `locationId`.
3. Create/reuse local client.
4. Create/reuse local integration.
5. Persist OAuth token.
6. Register RVPay as the HighLevel Custom Payment Provider for that location using the correct v3 contract.
7. Supply provider metadata so it is actually registered/displayed by HighLevel.
8. Persist the corresponding local provider configuration.
9. Remain idempotent.
10. Test the flow.

Assume the HighLevel platform row already exists. Never create it.

## Critical v3 contract check
Verify before editing:

The v3 Custom Provider "Create new integration" contract documents:
`POST /payments/custom-provider/provider`

with:
- `Version: v3`
- `locationId` as a required query parameter
- JSON body including:
  - `name`
  - `description`
  - `paymentsUrl`
  - `queryUrl`
  - `imageUrl`
  - `supportsSubscriptionSchedule`

Do not assume the current `CreateProviderAssociation` request is correct.

Also verify the exact v3 semantics for:
`POST/GET /payments/custom-provider/connect`

Do not confuse provider association with provider connection/configuration.

## Existing environment configuration
Use the existing fields:
- `HIGHLEVEL_PROVIDER_NAME`
- `HIGHLEVEL_PROVIDER_DESCRIPTION`
- `HIGHLEVEL_PROVIDER_IMAGE_URL`
- `HIGHLEVEL_PAYMENT_URL`
- `HIGHLEVEL_QUERY_URL`

Map them to the v3 request fields. Do not invent another config system.

## HTTP requirements
Affected GHL payment-provider requests must use:
- Bearer access token
- `Version: v3`
- correct method/path
- correct query parameters
- correct JSON body
- safe error handling

Never log access tokens, refresh tokens, client secrets, API keys, or authorization codes.

## Idempotency/error handling
Do not classify every 400/422 as "already exists".
Use v3 semantics/response information to distinguish:
- existing association/config
- invalid request
- unauthorized token
- missing permissions
- server/unexpected errors

For an existing configuration:
- fetch it when appropriate
- validate it
- persist real remote metadata
- do not overwrite valid metadata with empty/default values

Avoid unnecessary API-key regeneration when a valid local configuration already exists.

## DB/repository work
Inspect the actual Clients schema first.

If the local provider-config schema lacks a required field, make the smallest Clients-only migration/query/repository/model change.

Follow repository generation rules. Never hand-edit generated SQLC output.

## Agency/company installs
Do not confuse:
- company/agency ID
- installing user ID
- location/sub-account ID

Provider registration for a sub-account must use the correct location ID.

If the existing callback does not support bulk/multi-location installation, do not implement broad bulk behavior unless the current contract/tests require it. Document the limitation.

## Tests
Add/update focused tests for:
1. v3 association method/path.
2. `locationId` query placement.
3. `Version: v3`.
4. provider metadata body.
5. success.
6. existing association.
7. configuration create/fetch.
8. unauthorized.
9. malformed/unexpected responses.
10. single OAuth code exchange.
11. stateless Marketplace callback client/integration provisioning.
12. provider registration receives token response locationId.
13. local provider metadata persistence.
14. credential-safe errors/logs.

Use existing test patterns.

## Verification
Run:
- `go test ./clients/... -count=1`
- `go vet ./clients/...`
- `gofmt` on changed hand-written Go files

Do not fix unrelated failures.

## Marketplace dashboard report
At the END of the implementation, add a section to the checkpoint:

`Marketplace Dashboard — Required/Verified Configuration`

Separate:
- Required
- Recommended
- Automatically handled by API

Check v3 documentation for:
- payment scopes
- OAuth redirect URL
- Custom Payment Provider capability/module
- webhook configuration if relevant
- public HTTPS payment/query endpoints

Do not claim dashboard requirements without v3 evidence.

## Completion files
Update `.clinecheck.md` with implementation details and tests.

APPEND to `.service-checkpoint.md`:
`Agent 04 — Implementation`
Include:
- problem
- implementation
- files changed
- DB changes
- tests
- v3 endpoints/contracts
- dashboard requirements
- remaining risks

Do not overwrite Agent 03.

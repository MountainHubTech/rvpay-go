# GHL Payment Provider — Installation & Uninstallation Audit

| Field | Value |
| --- | --- |
| Type | Audit / Implementation Ticket |
| Component | Clients service (`clients/`) |
| Scope | HighLevel Marketplace install/uninstall as a Custom Payment Provider |
| Status | Installation mostly implemented; uninstallation missing |
| Owner | Clients service |

---

## Description

This ticket compares the current Clients implementation with the HighLevel
Marketplace protocol for installing an app designated as a **Custom Payment
Provider**, and defines the missing uninstallation flow.

### Installation — HighLevel required steps vs current implementation

| # | HighLevel requirement | Current RVPay status | Gap / action |
| --- | --- | --- | --- |
| 1 | Marketplace app configured with payment scopes, Redirect URL, Webhook URL, client keys, Custom Payment Provider module, provider profile, and public payment/query URLs. | Documented as manual dashboard configuration (`.service-checkpoint.md`, Agent 05). | No code change; confirm in the live Marketplace dashboard. |
| 2 | User installs the app; HighLevel redirects to the Redirect URL with `code` (Marketplace callbacks normally have no `state`). | Implemented: `GET /oauth/callback` → `oauth.Service.HandleCallback` stateless branch (`clients/oauth/service.go:201`). | None. |
| 3 | Exchange the authorization code for a Location OAuth token and obtain `locationId`. | Implemented exactly once (`clients/oauth/service.go:245`); token persisted replace-or-insert (`:496`). | Open question: docs show snake_case token form fields; code uses camelCase (test-locked). Verify on a live install. |
| 4 | Resolve/provision the RVPay tenant for the location (client + integration). | Implemented in OAuth callback (`:271`–`:297`) and INSTALL webhook (`clients/webhooks/service.go:333`). Client is created `ACTIVE`; integration `CREATED` → `ACTIVE`. | Reinstall gap: an existing client left inactive is **not reactivated**, and an existing non-`CREATED` integration is reused without being set `ACTIVE`. |
| 5 | Register the provider association with metadata (`name`, `description`, `imageUrl`, `paymentsUrl`, `queryUrl`, `supportsSubscriptionSchedule`). | Implemented: `POST /payments/custom-provider/provider?locationId=<id>` in `clients/oauth/service.go:686`. | None. |
| 6 | Enable Custom Payment Provider capabilities. | Implemented best-effort: `PUT /payments/custom-provider/capabilities` (`clients/oauth/service.go:673`). | None. |
| 7 | Confirm the base provider configuration exists before pushing keys. | Implemented with bounded retry on `GET /payments/custom-provider/connect` (`clients/oauth/service.go:724`). | None. |
| 8 | Push test/live provider keys. | Implemented from environment config: `POST /payments/custom-provider/connect` (`clients/oauth/service.go:786`). | Keys are optional; install still succeeds if not configured. |
| 9 | Persist the local provider configuration and mark the tenant active. | Implemented: `payment_provider_configs` create/update (`clients/oauth/service.go:838`); client created `ACTIVE`; integration activated. | Reinstall must also re-activate the **client** (see row 4). |
| 10 | User selects the provider as default in HighLevel UI (`Payments > Integrations`). | Manual step; no GHL API exists. | Operational step, not code. |
| 11 | Handle the INSTALL webhook, which may arrive before or after the OAuth callback. | Implemented: signature verification, tenant provisioning, config create/reuse, remote provider reconciliation (`clients/providers/highlevel_webhook.go:297`, `:388`). | Reactivates only the integration, not the client. |

**Client status field check:** `clients.status` exists as PostgreSQL enum
`client_status` (`clients/db/migrations/000001_init_schema.up.sql:3`) with
values `REGISTERED`, `ACTIVE`, `SUSPENDED`, `CLOSED`. Installation already sets
the client to `ACTIVE` on creation (`clients/oauth/service.go:274`,
`clients/webhooks/service.go:356`). The closest "inactive" state used by the
existing deactivate flow is `CLOSED` (`clients/service/clients_service.go:214`).

### Uninstallation — HighLevel required steps vs current implementation

| # | HighLevel requirement | Current RVPay status | Gap / action |
| --- | --- | --- | --- |
| 1 | HighLevel POSTs an `UNINSTALL` webhook. Location payload: `{"type":"UNINSTALL","appId":"...","locationId":"..."}`; agency payload uses `companyId` instead. The documented payload does not include `timestamp` or `webhookId`. | Not handled. `ParseEvent` requires `timestamp` and uses `webhookId` (`clients/providers/highlevel_webhook.go:87`). | Accept uninstall payloads without `timestamp`/`webhookId`; default the time and derive a stable event ID. |
| 2 | Verify the webhook signature. | Implemented for all HighLevel webhooks (`clients/providers/highlevel_webhook.go:41`). | None. |
| 3 | Resolve the tenant from `locationId`. | Non-INSTALL events resolve only through `payment_provider_configs.location_id` (`clients/webhooks/service.go:212`). | Resolve via `integrations.external_account_id` first (same as INSTALL), with config fallback. |
| 4 | Record the event idempotently. | Implemented, but depends on a non-empty `provider_event_id` (`clients/webhooks/service.go:239`). | Derive a deterministic ID when `webhookId` is absent. |
| 5 | Do not require a webhook subscription for the lifecycle event. | Non-INSTALL events require a `webhook_subscriptions` row (`clients/webhooks/service.go:266`); `RegisterWebhook` is a stub. | Exempt `UNINSTALL` (and lifecycle events) like `INSTALL`. |
| 6 | Deactivate the tenant. | **Not implemented.** `handleIntegrationUninstalled` is a log-only stub (`clients/providers/highlevel_webhook.go:419`). | Set client status `ACTIVE` → `CLOSED` (inactive) and integration status `ACTIVE` → `REVOKED`. |
| 7 | Clean up remote state. | HighLevel removes its remote provider association on uninstall; RVPay keeps local rows. | Keep local data non-destructive (no deletes); only change statuses. |
| 8 | Acknowledge the webhook. | Only after successful parse/resolution/dispatch. | Ensure uninstall reaches dispatch and returns `200`. |

**Net finding:** installation matches the HighLevel protocol except for
re-activation on reinstall. Uninstallation is effectively absent: the webhook
cannot be parsed, cannot be routed without a provider config/subscription, and
the handler does not change any status.

---

## Acceptance Criteria

### Installation hardening

- [ ] **Ensure the client is `ACTIVE` during installation, including reinstall.**
  - In the OAuth callback and INSTALL webhook, if the existing client status is not `ACTIVE`, call `ClientRepo.UpdateStatus(..., ClientStatusACTIVE)`.
  - Keep `Create(..., ClientStatusACTIVE)` for new clients.
- [ ] **Ensure the integration is `ACTIVE` during installation, including reinstall.**
  - In `processCallbackWithToken`, activate any existing non-`ACTIVE` integration, not only `CREATED`.
  - INSTALL webhook should continue reactivating non-`ACTIVE` integrations.
- [ ] **Add regression tests for reinstall.**
  - Existing `CLOSED` client becomes `ACTIVE` after OAuth callback and INSTALL.
  - Existing `REVOKED` integration becomes `ACTIVE` after install.
- [ ] **Document the token-field casing risk.**
  - Confirm snake_case vs camelCase against a live HighLevel install before changing.
  - Keep the single-exchange behavior unchanged.

### Uninstallation implementation

- [ ] **Parse HighLevel `UNINSTALL` payloads.**
  - Treat `timestamp` as optional and default it to the server receive time.
  - Treat `webhookId` as optional; derive a deterministic ID (e.g. `type + appId + locationId`) for idempotency.
- [ ] **Route uninstall events like install events.**
  - Resolve the integration by `integrations.external_account_id = locationId` first, then `payment_provider_configs.location_id`.
  - Skip the `webhook_subscriptions` requirement for `UNINSTALL`/`integration.uninstalled`.
- [ ] **Implement the uninstall status transition.**
  - Add `clientsRepo` to `HighLevelWebhookDispatcher` (constructor or setter) and wire it in `cmd/grpc-service/main.go`.
  - On uninstall, set client status `ACTIVE` → `CLOSED` and integration status `ACTIVE` → `REVOKED`.
  - Make the handler idempotent; a repeated uninstall must not fail or delete rows.
- [ ] **Keep uninstall non-destructive.**
  - Do not delete `oauth_tokens`, `integrations`, or `payment_provider_configs`.
  - Rely on HighLevel to remove its remote provider association.
- [ ] **Optionally treat related lifecycle events as deactivation.**
  - `oauth.revoked` and `provider.disconnected` may use the same deactivation path.
  - Document the chosen behavior.
- [ ] **Add uninstall tests.**
  - Parse a location-level uninstall payload without `timestamp`/`webhookId`.
  - Assert client `CLOSED` and integration `REVOKED`; duplicate event is idempotent.
  - Assert unknown location returns a clear error without changing other tenants.

### Verification

- [ ] `go test ./clients/... -count=1` passes.
- [ ] `go vet ./clients/...` is clean.
- [ ] No schema migration is required for this ticket; existing `client_status` and `integration_status` enums are sufficient.
- [ ] Update `clients/README.md` and `clients/.service-checkpoint.md` with the uninstall behavior and status mapping.

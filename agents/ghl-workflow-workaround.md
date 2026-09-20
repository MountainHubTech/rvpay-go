# Agent Task: Implement RVPay → HighLevel Inbound Workflow Webhook Delivery

## Objective

Implement a durable, location-specific outbound webhook integration from RVPay to a HighLevel Inbound Webhook workflow.

This is a fallback automation path for confirmed RVPay payments. It must not replace, remove, or alter the existing HighLevel custom payment-provider flow or the existing `record-payment` implementation.

The event must be emitted only after RVPay has authoritatively confirmed a successful payment and persisted that state.

## Required reading before making any changes

Read these repository documents completely before editing anything:

* `.clinecheck`
* `.clinerules`
* `.clineignore`
* `README.md`
* `docs/project-checkpoint.md`
* `.project-context.md`
* `admindashboard/.project-checkpoint.md`
* `admindashboard/.clinerules.md`
* `admindashboard/.clineignore.md`
* `admindashboard/.clinecheck.md`
* `admindashboard/README.md`
* `clients/.service-checkpoint.md`
* `clients/.clinerules.md`
* `clients/.clineignore.md`
* `clients/.clinecheck.md`
* `clients/README.md`
* `transactions/.service-checkpoint.md`
* `transactions/.clinerules.md`
* `transactions/.clineignore.md`
* `transactions/.clinecheck.md`
* `transactions/README.md`

If any listed file does not exist, report that fact before proceeding and do not invent a replacement.

Then inspect the relevant source files for:

* HighLevel OAuth and integration persistence
* HighLevel custom-provider registration
* HighLevel payment initiation context handling
* HighLevel order and transaction ID persistence
* Transactions public deposit creation
* PawaPay deposit callback handling
* Deposit/payment state transitions
* Existing background workers
* Existing outbox/event infrastructure
* Existing HTTP client abstractions
* Existing database migrations
* Existing authentication and authorization middleware
* Existing logging and error-handling conventions
* Existing tests for Clients and Transactions services

At minimum, locate and read the complete implementations of:

* `clients/providers/highlevel.go`
* `clients/providers/highlevel_webhook.go`
* `clients/oauth/service.go`
* `clients/http/oauth_handler.go`
* the code that persists HighLevel integrations
* the code that receives or persists HighLevel payment initiation data
* the Transactions deposit creation handler/service
* the PawaPay callback handler/service
* the deposit status transition logic
* any existing outbox/event worker implementation
* the database schema and migration files for integrations and deposits

Do not assume paths. Find the actual current paths and record them in the checkpoint.

## Functional requirements

### 1. Location-specific configuration

Add a location-specific HighLevel inbound workflow webhook URL associated with the existing HighLevel integration record.

Use the existing HighLevel location identity and integration ownership model. Do not create a global webhook URL configuration.

Prefer a database-backed configuration field or an existing integration-settings mechanism.

Support:

* setting the URL;
* enabling/disabling delivery;
* rotating the URL;
* validating HTTPS URLs;
* preventing cross-location access;
* avoiding exposure of the full URL in ordinary logs or responses.

If a suitable authenticated configuration endpoint already exists, extend it. Otherwise implement the smallest secure endpoint consistent with the existing Clients service conventions.

Do not create a public unauthenticated configuration endpoint.

### 2. Payment-completed event

Emit an event named:

`rvpay.payment.completed`

The payload must be JSON and include, where available:

* `event`
* `eventId`
* `paymentId`
* `depositId`
* `orderId`
* `transactionId`
* `locationId`
* `contactId`
* `customerEmail`
* `customerPhone`
* `amount`
* `currency`
* `status`
* `provider`
* `providerTransactionId`
* `productName`
* `idempotencyKey`
* `occurredAt`

Use persisted and verified RVPay data as the source of truth. Do not trust a browser-supplied paid status, amount, or provider result.

Do not emit the event for pending, failed, cancelled, or reversed payments.

### 3. Durable delivery

Use the existing outbox/event infrastructure if present.

If no suitable outbox exists, add the smallest implementation necessary to guarantee:

* event persistence;
* retry after transient failure;
* bounded attempts;
* delivery status;
* last error;
* next retry time;
* successful delivery timestamp;
* idempotent event identity.

Create the outbox event in the same database transaction as the successful payment-state transition whenever the current architecture permits it.

Do not make the PawaPay callback handler wait for HighLevel’s webhook response.

### 4. HTTP delivery

Send a POST request with:

* `Content-Type: application/json`
* `User-Agent: RVPay/<version>`
* `X-RVPay-Event: rvpay.payment.completed`
* `X-RVPay-Event-Id: <event id>`

Use a bounded timeout.

Support retryable handling for network errors and appropriate 5xx responses.

Do not retry permanent 4xx errors indefinitely.

Do not log:

* bearer tokens;
* webhook secrets;
* full webhook URLs;
* raw customer secrets.

If the project already has a signing convention, follow it. Otherwise add optional HMAC-SHA256 signing only if it fits the existing security architecture without expanding the task unnecessarily.

### 5. Idempotency

Repeated PawaPay callbacks must not create duplicate successful-payment events.

Repeated delivery attempts must reuse the same event ID and payload.

The implementation must not mark an event delivered before the HighLevel endpoint returns a successful response.

### 6. Observability

Add structured logs containing only safe identifiers:

* event ID;
* payment/deposit ID;
* HighLevel location ID;
* order ID;
* HTTP status;
* attempt count;
* delivery result.

Never log the webhook URL or secret.

## Explicit non-goals

Do not:

* modify the HighLevel `record-payment` request;
* modify OAuth scopes;
* change Marketplace installation behavior;
* change custom-provider registration;
* call undocumented HighLevel dashboard endpoints;
* implement the custom-provider `payment.captured` protocol;
* change the PawaPay payment logic beyond the event emission hook;
* redesign the existing architecture;
* add unrelated AWS, Docker, frontend, or dashboard work;
* change database ownership boundaries between Clients and Transactions;
* expose payment secrets to the browser.

## Required tests

Add or update tests for:

* successful payment creates exactly one outbox event;
* repeated provider callback does not create duplicate events;
* non-successful payment creates no completed event;
* correct HighLevel location integration is selected;
* wrong-location access is rejected;
* invalid webhook URL is rejected;
* disabled webhook configuration does not deliver;
* successful HTTP response marks the event delivered;
* timeout/network error schedules retry;
* permanent 4xx response is handled without infinite retry;
* retry uses the same event ID;
* payload contains persisted order/contact/location/payment data;
* secrets and webhook URLs are absent from logs.

Run the project’s applicable:

* unit tests;
* integration tests;
* build;
* `go vet`;
* formatting checks;
* repository validation commands.

Do not report tests as passing unless they were actually run.

## Required documentation updates

Append, do not replace, the following files:

* `.clinecheck`
* `.project-checkpoint.md`
* `docs/project-checkpoint.md`
* `README.md`

If the repository uses a different established checkpoint file in addition to the above, append to that file too.

The appended checkpoint entry must include:

* date;
* agent name;
* exact files read;
* exact files changed;
* database migrations added;
* API/configuration changes;
* event payload schema;
* retry/idempotency behavior;
* tests executed and results;
* build/vet results;
* known limitations;
* remaining manual HighLevel setup steps;
* next recommended task.

Also update the relevant project context document by appending the new webhook configuration, event flow, and operational behavior. Do not delete or rewrite previous checkpoint entries.

## Completion criteria

The task is complete only when:

1. RVPay can store a HighLevel inbound workflow URL per location.
2. A confirmed successful payment creates one durable `rvpay.payment.completed` event.
3. A worker delivers the event asynchronously.
4. Delivery retries safely.
5. Duplicate provider callbacks do not duplicate the event.
6. Tests cover the required behavior.
7. All required checkpoint and documentation updates are appended.
8. No unrelated files or behavior are changed.

Before finishing, provide a concise summary of:

* files changed;
* migrations;
* endpoints/configuration added;
* tests run;
* validation results;
* manual HighLevel steps still required.

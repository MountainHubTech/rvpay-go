# Agent Task: Implement RVPay → HighLevel Inbound Workflow Webhook Delivery

## Objective

Implement a durable, environment-configured outbound webhook integration from RVPay to an existing HighLevel Inbound Webhook workflow.

This is a fallback automation path for confirmed RVPay payments.

It must **not** replace, remove, or alter the existing HighLevel custom payment-provider flow or the existing `record-payment` implementation.

The event must be emitted only after RVPay has authoritatively confirmed a successful payment and persisted that state.

The HighLevel Inbound Webhook workflow has already been manually created and configured by the developer.

The agent is responsible for implementing the **RVPay side** of the integration.

The intended flow is:

```text
RVPay
  │
  │ POST JSON
  ▼
┌───────────────────────────────┐
│ HighLevel: Inbound Webhook    │
└───────────────┬───────────────┘
                │
                ▼
       Create/Update Contact
                │
                ▼
          If/Else #1
       event == rvpay.payment.completed
                │
                ▼
          If/Else #2
          status == paid
                │
                ▼
          If/Else #3
      payment already processed?
                │
                ▼
       Update Contact
                │
                ▼
    Add "RVPay Payment Completed"
                │
                ▼
       Send confirmation
                │
                ▼
         Send SMS
```

The HighLevel workflow itself has already been configured manually.

Do **not** attempt to create, modify, or discover the HighLevel workflow through an API.

Do **not** add the currently deferred Internal Notification or RVPay outbound webhook steps.

The actual HighLevel Inbound Webhook URL must be supplied to RVPay through:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

That value must ultimately be supplied to the production runtime through AWS Secrets Manager using the repository's existing AWS/ECS secret-injection architecture.

---

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
* Existing configuration/environment loading
* Existing AWS/ECS/Secrets Manager configuration
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
* the existing application configuration/environment loading code
* the existing AWS/ECS task definition or deployment configuration that injects Secrets Manager values

Do not assume paths.

Find the actual current paths and record them in the checkpoint.

Before implementing anything, understand how the current Clients and Transactions services communicate and where the authoritative payment/order/customer/integration information exists.

Do not create duplicate data stores merely to make this feature easier.

---

## Important HighLevel workflow context

The HighLevel workflow has already been manually created.

The current workflow is:

```text
Inbound Webhook
      ↓
Create/Update Contact
      ↓
If/Else #1
    event == rvpay.payment.completed
      ↓
If/Else #2
    status == paid
      ↓
If/Else #3
    payment already processed?
      ↓
Update Contact
      ↓
Add "RVPay Payment Completed" tag
      ↓
Send confirmation email
      ↓
Send SMS
```

The current HighLevel If/Else UI uses:

* Action Name
* Scenario Recipe
* Branches
* None branch

The RVPay implementation does **not** need to reproduce or understand this internal UI structure.

The only requirement from RVPay is that it sends the agreed JSON event contract.

The workflow currently relies on these inbound fields in particular:

```text
event
status
paymentId
customerEmail
customerPhone
firstName
lastName
amount
currency
occurredAt
```

The broader payload also provides:

```text
eventId
depositId
orderId
transactionId
locationId
contactId
provider
providerTransactionId
productName
idempotencyKey
```

There is currently **NO**:

* HighLevel Internal Notification action
* HighLevel outbound Custom Webhook action
* RVPay fulfillment callback

Those are intentionally deferred.

Do not implement them as part of this task.

---

## Functional requirements

### 1. HighLevel inbound webhook URL configuration

The HighLevel inbound workflow URL must be configured as an **environment-level secret/configuration value**.

Use exactly:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

Do **not** store the HighLevel inbound webhook URL in the database.

Do **not** create a location-specific webhook URL database field.

Do **not** create a database-backed per-location webhook URL configuration for this task.

Do **not** create a public configuration endpoint for the webhook URL.

Do **not** expose the webhook URL through the dashboard.

Do **not** hard-code the webhook URL in source code.

Do **not** commit the actual webhook URL to Git.

Do **not** put the actual webhook URL into:

* source code;
* tests;
* README;
* checkpoint files;
* project context files;
* logs;
* API responses;
* metrics;
* tracing attributes.

The actual URL must be supplied at runtime through:

```text
AWS Secrets Manager
        ↓
ECS/runtime secret injection
        ↓
HIGHLEVEL_INBOUND_WEBHOOK_URL
        ↓
RVPay webhook delivery worker
        ↓
HighLevel Inbound Webhook
```

Inspect the existing AWS/ECS/Secrets Manager architecture before implementing this.

Follow the repository's existing secret-injection pattern.

Do not create an unrelated secrets-management architecture.

If the existing infrastructure uses a shared JSON secret, add the appropriate key according to the established pattern.

If the existing infrastructure uses individual Secrets Manager values, follow that pattern.

Determine the correct approach by inspecting the repository.

The application must read `HIGHLEVEL_INBOUND_WEBHOOK_URL` through the repository's existing configuration mechanism.

Validate that the configured value is an HTTPS URL where the existing configuration architecture permits validation.

A missing webhook URL must **not** cause an otherwise successful RVPay payment to become unsuccessful.

If the webhook configuration is unavailable, the payment remains authoritative and successful; only downstream HighLevel automation delivery is unavailable.

The agent must document:

* the environment variable name;
* how it is loaded by the application;
* how production receives it from AWS Secrets Manager;
* which service/task receives it;
* how the value is rotated;
* whether an ECS deployment/restart is required for the new value to be picked up.

Do not put the actual URL in repository documentation.

---

### 2. Environment scope and HighLevel location identity

The webhook URL is environment-scoped for this implementation.

The event itself must still contain the authoritative HighLevel location ID:

```text
locationId
```

The `locationId` must come from the existing RVPay payment/integration context.

Do not hard-code:

* the test location ID;
* an agency ID;
* a location ID from environment configuration;
* a location ID supplied by an untrusted browser request when authoritative persisted data exists.

The event's `locationId` must identify the HighLevel sub-account/location associated with the successful RVPay payment.

Do not redesign the current multi-location architecture to introduce per-location webhook URL routing.

If the current architecture creates a future limitation because one environment-level webhook URL cannot represent multiple independently configured HighLevel workflows or locations, document that limitation rather than expanding the scope of this task.

---

### 3. Payment-completed event

Emit an event named:

```text
rvpay.payment.completed
```

The outbound JSON payload must use this **exact field contract**:

```json
{
  "event": "rvpay.payment.completed",
  "eventId": "test-event-001",
  "paymentId": "test-payment-001",
  "depositId": "test-deposit-001",
  "orderId": "6aa60c9565a70758a0d4fbc8",
  "transactionId": "test-transaction-001",
  "locationId": "rVBsqXAKXlYLz0C96kA8",
  "contactId": "TL2jQXT39zCqP8dVZsKx",
  "customerEmail": "gilbert@mountainhub.africa",
  "customerPhone": "654131027",
  "firstName": "Gilbert",
  "lastName": "Test",
  "amount": 2,
  "currency": "XAF",
  "status": "paid",
  "provider": "pawapay",
  "providerTransactionId": "test-pawapay-001",
  "productName": "RVPay Test Product",
  "idempotencyKey": "test-payment-001",
  "occurredAt": "2026-09-18T12:00:00Z"
}
```

The values above are an **example/test payload**, not production data.

The structure and field names are the contract.

Do not rename these fields.

Do not convert them to snake_case.

Do not create an alternative payload contract.

Do not omit fields merely because they are inconvenient to obtain.

If a field cannot currently be populated, trace the existing domain model and determine whether authoritative data already exists elsewhere in the application.

Only after that investigation should the agent report a genuine data-model limitation.

The completed event must use:

```text
event    = rvpay.payment.completed
status   = paid
provider = pawapay
```

The remaining values must come from authoritative persisted RVPay data.

Do not use:

* browser-supplied payment status;
* browser-supplied amount;
* browser-supplied provider result;
* arbitrary webhook request data;
* unverified client-side payment confirmation

as the source of truth.

Do not emit the completed event for:

* pending;
* failed;
* cancelled;
* reversed;
* otherwise unsuccessful payments.

The event must be created only after RVPay has authoritatively transitioned/persisted the payment as successful according to the existing payment state model.

---

### 4. Event field sourcing

Populate:

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
* `firstName`
* `lastName`
* `amount`
* `currency`
* `status`
* `provider`
* `providerTransactionId`
* `productName`
* `idempotencyKey`
* `occurredAt`

from the existing authoritative RVPay data model.

Do not fabricate values.

Do not use test values in production code.

The agent must trace how each field can be obtained from the existing system.

In particular, investigate the persisted relationship between:

```text
HighLevel location
        ↓
HighLevel order
        ↓
RVPay payment/deposit
        ↓
PawaPay transaction
        ↓
customer/contact
```

Pay particular attention to:

* HighLevel `locationId`;
* HighLevel `contactId`;
* HighLevel `orderId`;
* RVPay `transactionId`;
* customer email;
* customer phone;
* customer first name;
* customer last name;
* payment ID;
* deposit ID;
* PawaPay provider transaction ID;
* product name;
* idempotency key.

These values must come from the appropriate persisted RVPay context rather than from the browser.

If the current implementation does not persist a particular field, do not silently substitute an unrelated value.

Determine whether the field can be obtained through existing relationships.

If it genuinely cannot be populated, document:

1. the missing field;
2. the current source of related data;
3. why it cannot currently be derived;
4. the smallest future change required.

Do not expand this task into an unrelated domain-model redesign.

---

### 5. Durable delivery

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

Do not make the PawaPay callback handler wait for HighLevel's webhook response.

The desired flow is:

```text
PawaPay callback
      ↓
RVPay verifies/handles callback
      ↓
RVPay persists successful payment state
      ↓
RVPay persists outbound event/outbox record
      ↓
PawaPay callback completes
      ↓
Background worker
      ↓
POST HighLevel Inbound Webhook
```

Do not make HighLevel availability part of the critical payment-success path.

The worker must be able to continue delivery after an application restart.

A transient HighLevel outage must not lose a payment-completed event.

---

### 6. HTTP delivery

Send an HTTP `POST` request to:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

The request body must be the exact event JSON contract defined above.

Use:

```text
Content-Type: application/json
Accept: application/json
User-Agent: RVPay/<version>
X-RVPay-Event: rvpay.payment.completed
X-RVPay-Event-Id: <event id>
```

Use a bounded timeout.

Use the existing HTTP client abstraction if one exists.

Do not create a second unrelated HTTP client architecture.

Support retryable handling for:

* network errors;
* connection errors;
* request timeouts;
* appropriate HTTP 5xx responses.

Do not retry permanent HTTP 4xx responses indefinitely.

The implementation must not mark an event delivered before the HighLevel endpoint returns a successful HTTP response.

A successful HighLevel response means the HTTP request completed with an appropriate success status according to the implementation's HTTP semantics.

Do not assume that a particular response body is required unless the HighLevel endpoint actually requires one.

Do not log:

* OAuth bearer tokens;
* OAuth secrets;
* PawaPay credentials;
* webhook secrets;
* the full HighLevel webhook URL;
* raw customer secrets.

Do not log the complete event payload by default if that would expose unnecessary customer PII.

If the project already has a signing convention, follow it.

Otherwise, do not add an unnecessary authentication/signing system to this task unless the existing security architecture makes it appropriate.

The HighLevel inbound webhook URL itself must be treated as secret configuration.

---

### 7. Idempotency

Repeated PawaPay callbacks must not create duplicate successful-payment events.

Repeated delivery attempts must reuse:

* the same `eventId`;
* the same logical event;
* the same `idempotencyKey`;
* the same payload.

Do not generate a new event ID for every delivery attempt.

Do not generate a new idempotency key for every retry.

The event ID must be stable for the logical payment-completed event.

The implementation must not mark an event delivered before successful HTTP delivery.

The HighLevel workflow contains its own duplicate-processing check using `paymentId`.

That HighLevel duplicate check is **not** the primary RVPay idempotency mechanism.

RVPay remains responsible for:

* creating one logical completed-payment event;
* persisting that event;
* preserving its identity;
* retrying it safely.

The implementation must protect against concurrent duplicate callback handling where the current database architecture permits this.

Use the repository's existing transaction/unique-constraint/idempotency conventions where available.

Do not invent an unrelated idempotency framework.

---

### 8. Retry behavior

Use the existing worker/outbox retry conventions if available.

If none exist, implement a small bounded retry policy appropriate to the current application.

Retry transient failures such as:

* timeout;
* network failure;
* connection failure;
* HTTP 5xx.

Do not endlessly retry ordinary permanent 4xx errors.

Persist enough state to allow delivery to resume after process restart.

The same event ID must be used for all retries.

The same idempotency key must be used for all retries.

The same event payload must be used for all retries.

Do not create a new logical payment event merely because delivery failed.

If the worker receives a permanent 4xx response, record the failure and stop retrying according to the bounded retry policy.

Do not turn a HighLevel configuration problem into an infinite background retry loop.

---

### 9. Observability

Add structured logs containing only safe operational identifiers such as:

* event ID;
* payment ID;
* deposit ID;
* HighLevel location ID;
* order ID;
* HTTP status;
* attempt count;
* delivery result;
* retry scheduling result.

Never log:

* the actual HighLevel webhook URL;
* webhook secrets;
* OAuth tokens;
* PawaPay credentials;
* customer secrets.

Avoid logging unnecessary customer PII.

If logging an error returned from the HTTP client, ensure that the configured webhook URL is not embedded in the error output.

If configuration validation fails, log the configuration variable name:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

but never log its value.

---

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
* expose payment secrets to the browser;
* create a HighLevel workflow through an API;
* modify the existing manually created HighLevel workflow;
* create a HighLevel Internal Notification action;
* create a HighLevel outbound Custom Webhook action;
* create an RVPay fulfillment callback endpoint;
* implement a HighLevel → RVPay callback;
* store the HighLevel inbound webhook URL in the database;
* create a public API endpoint for configuring the HighLevel webhook URL;
* hard-code the HighLevel webhook URL;
* put the actual HighLevel webhook URL in source control;
* put the actual HighLevel webhook URL in documentation or checkpoint files;
* create per-location webhook URL configuration;
* introduce a new secrets-management architecture;
* expose `HIGHLEVEL_INBOUND_WEBHOOK_URL` through an API;
* put the actual webhook URL into automated tests;
* log the actual webhook URL.

The HighLevel webhook URL must be supplied only through:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

and the production value must be injectable through the existing AWS Secrets Manager/ECS configuration mechanism.

---

## Required tests

Add or update tests for:

* successful payment creates exactly one outbox event;
* repeated provider callback does not create duplicate events;
* concurrent duplicate callback handling does not create duplicate completed events where the current transaction architecture supports this;
* non-successful payment creates no `rvpay.payment.completed` event;
* correct HighLevel `locationId` is selected from authoritative persisted payment/integration context;
* an incorrect/untrusted location value is not substituted for the authoritative location;
* invalid `HIGHLEVEL_INBOUND_WEBHOOK_URL` configuration is rejected or safely disabled according to the existing configuration conventions;
* missing `HIGHLEVEL_INBOUND_WEBHOOK_URL` does not cause a successful payment to become unsuccessful;
* disabled/unconfigured HighLevel delivery does not break successful payment processing;
* successful HTTP response marks the event delivered;
* timeout/network error schedules retry;
* HTTP 5xx schedules retry;
* permanent 4xx response is handled without infinite retry;
* retry uses the same event ID;
* retry uses the same idempotency key;
* retry uses the same payload/event identity;
* payload contains the exact agreed field names;
* payload contains the authoritative persisted order/contact/location/payment data;
* `event` is exactly `rvpay.payment.completed`;
* `status` is exactly `paid` for completed events;
* `provider` is correctly populated as `pawapay`;
* customer email is sourced from authoritative persisted data;
* customer phone is sourced from authoritative persisted data;
* first name is sourced from authoritative persisted data;
* last name is sourced from authoritative persisted data;
* amount is sourced from authoritative persisted payment data;
* currency is sourced from authoritative persisted payment data;
* provider transaction ID is sourced from authoritative PawaPay data;
* order ID is sourced from authoritative HighLevel/payment context;
* location ID is sourced from authoritative HighLevel integration/payment context;
* secrets and webhook URLs are absent from logs;
* the actual webhook URL is not present in source-controlled configuration or tests.

Run the project's applicable:

* unit tests;
* integration tests;
* build;
* `go vet`;
* formatting checks;
* repository validation commands.

Do not report tests as passing unless they were actually run.

---

## AWS Secrets Manager requirements

Inspect the existing AWS deployment configuration before making changes.

Determine exactly how the current production ECS/runtime environment receives secrets from AWS Secrets Manager.

Use the existing architecture.

The production runtime must ultimately provide:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

to the appropriate RVPay service.

Do not create a second secrets-management system.

Do not hard-code:

* AWS account IDs;
* secret values;
* the actual HighLevel URL;
* environment-specific credentials.

If infrastructure changes are required to expose the existing Secrets Manager value to the application, make only the minimal changes necessary.

Inspect existing:

* CloudFormation;
* ECS task definitions;
* environment configuration;
* Secrets Manager references;
* IAM permissions;
* task execution/task roles;
* service-specific secret injection.

Determine whether the current task role or execution configuration already permits the required secret access.

Do not broaden IAM permissions unnecessarily.

If an existing Secrets Manager secret contains multiple key/value pairs, follow the established project pattern.

If an individual secret is used for each value, follow that pattern.

The required runtime configuration is:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

Document:

* the expected secret/key name;
* where the secret is stored;
* how ECS/runtime receives it;
* which service consumes it;
* how an operator rotates/changes it;
* whether a new ECS deployment/restart is required after changing it;
* any required IAM permission changes.

Do not put the actual HighLevel webhook URL into any repository file.

If the existing infrastructure cannot safely support this without a major redesign, stop and report the blocker instead of inventing a new AWS architecture.

---

## Database requirements

Do not add a database column for:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

The webhook URL is environment-level secret configuration for this task.

Database changes are permitted only where required for the durable outbound event/outbox mechanism.

If an existing outbox/event mechanism is suitable, extend it rather than creating duplicate infrastructure.

If database changes are required:

* create the appropriate migration using existing project conventions;
* do not modify existing migrations;
* make the migration safe and deterministic;
* document the migration;
* explain exactly what it stores.

Do not change database ownership boundaries between Clients and Transactions.

The database must not become the storage location for the HighLevel inbound webhook URL.

---

## HighLevel workflow contract

The manually configured HighLevel workflow expects the following inbound webhook fields:

```text
event
eventId
paymentId
depositId
orderId
transactionId
locationId
contactId
customerEmail
customerPhone
firstName
lastName
amount
currency
status
provider
providerTransactionId
productName
idempotencyKey
occurredAt
```

The workflow currently uses:

```text
event
status
paymentId
customerEmail
customerPhone
firstName
lastName
amount
currency
occurredAt
```

The agent must ensure that its event payload supports these workflow operations.

The event must use the exact field names above.

Do not change the event contract merely to fit the current implementation.

If the current RVPay data model cannot populate a required field, investigate the existing data relationships first.

If the field genuinely cannot be populated, document the limitation rather than silently substituting unrelated data.

---

## Manual HighLevel setup

The developer has already completed the HighLevel workflow setup.

Do not ask the developer to recreate the workflow.

Do not create additional HighLevel actions.

The current manually configured workflow performs:

```text
Inbound Webhook
    ↓
Create/Update Contact
    ↓
If/Else:
    event = rvpay.payment.completed
    ↓
If/Else:
    status = paid
    ↓
Duplicate check using paymentId
    ↓
Update Contact fields
    ↓
Add RVPay Payment Completed tag
    ↓
Send confirmation email
    ↓
Send SMS
```

The actual HighLevel webhook URL will be provided through:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

The agent must not require the developer to place the URL into source code.

The agent must not include the actual URL in documentation or checkpoint files.

The agent does not need to create any HighLevel workflow actions.

The agent does not need to modify the HighLevel workflow.

---

## Required documentation updates

Append, do not replace, the following files:

* `.clinecheck`
* `.project-checkpoint.md`
* `README.md`
* `clients/.service-checkpoint.md`
* `clients/.clinecheck.md`
* `clients/README.md`
* `transactions/.service-checkpoint.md`
* `transactions/.clinecheck.md`
* `transactions/README.md`

If the repository uses a different established checkpoint file in addition to the above, append to that file too.

The appended checkpoint entry must include:

* date;
* agent name;
* exact files read;
* exact files changed;
* database migrations added;
* API/configuration changes;
* `HIGHLEVEL_INBOUND_WEBHOOK_URL` configuration;
* AWS Secrets Manager/ECS secret injection changes;
* IAM changes, if any;
* event payload schema;
* event source/state-transition hook;
* authoritative source for each important event field;
* retry behavior;
* idempotency behavior;
* outbox/worker behavior;
* tests executed and results;
* build/vet results;
* known limitations;
* remaining manual HighLevel setup steps;
* remaining manual AWS secret configuration steps;
* next recommended task.

Also update the relevant project context document by appending the new:

* webhook configuration;
* event flow;
* AWS secret behavior;
* event contract;
* event field sourcing;
* retry behavior;
* idempotency behavior;
* worker behavior;
* operational behavior.

Do not delete or rewrite previous checkpoint entries.

Do not record the actual HighLevel webhook URL in the checkpoint.

Use only:

```text
HIGHLEVEL_INBOUND_WEBHOOK_URL
```

when referring to the secret.

---

## Completion criteria

The task is complete only when:

1. RVPay reads `HIGHLEVEL_INBOUND_WEBHOOK_URL` through the existing application configuration mechanism.

2. The HighLevel inbound webhook URL is not hard-coded.

3. The HighLevel inbound webhook URL is not stored in the database.

4. The actual HighLevel webhook URL is not present in source-controlled files.

5. Production can provide `HIGHLEVEL_INBOUND_WEBHOOK_URL` through AWS Secrets Manager using the existing ECS/runtime secret injection architecture.

6. Required IAM access exists, if needed, using the minimum necessary permissions.

7. A confirmed successful payment creates one durable `rvpay.payment.completed` event.

8. The event uses the exact agreed JSON field names and structure.

9. The event contains authoritative payment/order/customer/location information.

10. The event contains the authoritative HighLevel `locationId`.

11. The event contains the required contact information used by the existing HighLevel workflow.

12. The event contains the authoritative PawaPay provider transaction ID.

13. A worker delivers the event asynchronously.

14. The PawaPay callback does not wait for HighLevel webhook delivery.

15. Delivery retries safely.

16. Duplicate provider callbacks do not duplicate the logical event.

17. Delivery retries reuse the same event ID.

18. Delivery retries reuse the same idempotency key.

19. Delivery retries reuse the same event payload.

20. HighLevel availability does not determine whether RVPay considers the payment successful.

21. Permanent HTTP 4xx failures do not cause infinite retries.

22. Transient failures are retried according to the bounded retry policy.

23. Secrets and the actual webhook URL are not logged.

24. Tests cover the required behavior.

25. All required checkpoint and documentation updates are appended.

26. No unrelated files or behavior are changed.

27. The manually configured HighLevel workflow remains untouched.

28. No Internal Notification or Optional RVPay Webhook functionality is introduced.

Before finishing, provide a concise summary of:

* files changed;
* migrations;
* configuration added;
* AWS Secrets Manager changes;
* ECS/task-definition changes;
* IAM changes;
* event emission point;
* authoritative source of each important event field;
* exact event payload schema;
* outbox/worker behavior;
* retry behavior;
* idempotency behavior;
* tests run;
* validation results;
* manual AWS steps still required;
* manual HighLevel steps still required, if any.

Do not report anything as completed unless it was actually implemented and verified.

If a required piece cannot be implemented safely without a major architectural change, stop at that point and report the blocker rather than expanding the scope of this task.

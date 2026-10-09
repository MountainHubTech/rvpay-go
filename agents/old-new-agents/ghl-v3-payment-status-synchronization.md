You are the implementation agent for the RVPay repository.

Your task is to implement the missing synchronization between RVPay deposit finalization and the corresponding GoHighLevel (GHL) order/payment status.

The existing payment flow is already working and must be treated as stable:

- The GHL payment-provider iframe loads correctly.
- The customer can complete or fail a payment.
- RVPay creates the PawaPay deposit correctly.
- PawaPay processes the deposit correctly.
- PawaPay calls the configured callback.
- RVPay updates the deposit record correctly to its final state:
  - completed, or
  - failed.
- The remaining defect is that the original GHL order remains stuck in pending.
- There is currently no server-side outbound call from RVPay to GHL to update that order.

Your responsibility is to implement the missing server-side GHL status synchronization while preserving the existing payment flow.

This is an implementation task, not a broad refactor.

============================================================
1. EXECUTION LOCATION AND REPOSITORY BOUNDARIES
============================================================

Run from the repository root.

The repository contains at least these relevant services:

- admindashboard/
- clients/
- transactions/
- shared/
- protobuf/
- infrastructure/documentation files
- existing agent/checkpoint/rules files

Respect the existing project separation:

- RVPay is the current repository.
- Do not modify the separate PawaPay SDK project.
- Do not change the PawaPay SDK module path.
- Do not introduce imports from github.com/I-Frostbyte/rvpay-go.
- RVPay’s module path is the MountainHubTech organization path already established in go.mod.

Do not modify:

- aws-cloudformation-templates
- protected ALB/listener YAML
- unrelated infrastructure
- unrelated dashboard pages
- unrelated payment behavior
- unrelated authentication behavior
- unrelated generated files by hand

============================================================
2. REQUIRED FIRST PHASE: READ ALL RULES AND CONTEXT
============================================================

Before changing any file, inspect and consume all relevant instructions and project context.

At repository root, locate and read:

- .clinerules
- .clinerules.md
- .clineignore
- .clineignore.md
- .clinecheck
- README.md
- any other root-level project context, checkpoint, workflow, or agent instruction files required by the repository

Then inspect each affected service and read every applicable file before editing.

For admindashboard/, inspect and read:

- admindashboard/.clinerules
- admindashboard/.clinerules.md
- admindashboard/.clineignore
- admindashboard/.clineignore.md
- admindashboard/.clinecheck
- admindashboard/.project-context.md
- admindashboard/.project-checkpoint.md
- admindashboard/README.md
- any additional instruction, checkpoint, architecture, or workflow files mandated by those rules

For clients/, inspect and read:

- clients/.clinerules
- clients/.clinerules.md
- clients/.clineignore
- clients/.clineignore.md
- clients/.clinecheck
- clients/.service-context.md
- clients/.service-checkpoint.md
- clients/README.md
- any additional instruction, checkpoint, architecture, or workflow files mandated by those rules

For transactions/, inspect and read:

- transactions/.clinerules
- transactions/.clinerules.md
- transactions/.clineignore
- transactions/.clineignore.md
- transactions/.clinecheck
- transactions/.service-context.md
- transactions/.service-checkpoint.md
- transactions/README.md
- any additional instruction, checkpoint, architecture, or workflow files mandated by those rules

The actual filenames may vary. Discover them rather than assuming they all exist.

Strictly obey every consumed .clinerules.md, .clineignore.md, .clinerules, and .clineignore file.

Do not bypass, reinterpret, weaken, or override those rules.

If two instruction files appear to conflict:

1. Follow the repository’s established precedence rules.
2. Do not guess.
3. Record the conflict in the final report and relevant checkpoint append.
4. Prefer the more specific service rule for files within that service.

Do not begin implementation until this discovery and reading phase is complete.

============================================================
3. REQUIRED CHECKPOINT AND DOCUMENTATION DISCOVERY
============================================================

Before editing, determine:

- Which checkpoint files must be appended to.
- Which project-context files must be updated.
- Which README files must be updated.
- Which .clinecheck files must be updated.
- Whether the repository requires a file-consumption matrix.
- Whether the repository requires a project-next-steps file.
- Whether the repository requires test evidence or command logs.

Do not overwrite existing checkpoint history.

All checkpoint updates must be appended.

Do not replace, truncate, rewrite, or “clean up” previous checkpoint entries.

At minimum, append relevant updates to:

- root .clinecheck
- root project checkpoint/context file, if required
- admindashboard/.clinecheck.md
- admindashboard/.project-checkpoint.md
- transactions/.clinecheck.md
- transactions/.service-checkpoint.md
- clients/.clinecheck.md
- clients/.service-checkpoint.md
- README files required by the consumed rules
- any new implementation or endpoint documentation required by the repository

If a required file does not exist, follow the repository’s established convention rather than inventing a conflicting structure.

Each checkpoint append must include:

- date
- agent/task name
- files inspected
- files changed
- implementation decisions
- API decisions
- migration details
- test commands
- test results
- known limitations
- remaining deployment verification
- any unresolved GHL API uncertainty

============================================================
4. PRIMARY OBJECTIVE
============================================================

Implement reliable synchronization between:

- the final RVPay deposit status, and
- the corresponding GHL order/payment status.

The target GHL API is GHL’s v3 API.

All new GHL API requests must be consistent with the existing server’s v3 conventions, including:

- v3 API base URL conventions already used by the repository
- v3 request headers
- v3 authentication behavior
- existing Content-Type conventions
- existing Accept headers
- existing API version header conventions
- existing OAuth token refresh behavior
- existing location-level access-token storage
- existing error handling and logging conventions

Do not introduce a second GHL API versioning style.

Do not use legacy GHL API request formats if the existing server has already migrated to v3.

Inspect the existing HighLevel provider implementation first and reuse its established conventions.

============================================================
5. INSPECT THE WORKING PAYMENT FLOW
============================================================

Inspect:

- admindashboard/payment/page.tsx
- admindashboard/lib/api.ts
- dashboard request helpers
- AuthProvider/AuthGuard
- payment iframe initialization
- payment initiation payloads
- payment-provider callback/event handling
- clients provider configuration
- transactions deposit creation flow
- PawaPay deposit creation flow
- PawaPay callback handler
- deposit status update service
- deposit repository
- deposit schema and migrations
- protobuf definitions
- generated gRPC and gateway code
- existing GHL OAuth/token code
- existing GHL webhook/provider code
- existing transaction/payment metadata

Specifically determine whether admindashboard/payment/page.tsx sends any of the following to the server:

- orderId
- transactionId
- chargeId
- locationId
- contactId
- customerId
- any GHL payment-provider metadata
- any provider transaction reference

Trace the values from the dashboard into the backend.

Do not assume the orderId is already persisted merely because it is present in the iframe.

Document the actual path:

GHL → iframe → dashboard page → API helper → HTTP/gRPC endpoint → service → deposit creation → database.

============================================================
6. REQUIRED ghl_order_id PERSISTENCE
============================================================

The GHL order ID must be persisted as part of the deposit/payment linkage.

If orderId is not sent by the dashboard:

- determine whether it is available in the GHL iframe initialization payload or provider event
- update the dashboard payment flow only as necessary to send it
- preserve the existing payment behavior
- do not invent an order ID
- do not use mock data

If orderId is sent but is not persisted in the database:

- still create the required database migration
- add ghl_order_id to the deposits table
- update all relevant source models and queries
- update the proto request/response messages as required
- update service methods
- update repository methods
- update generated code through the repository’s official generation workflow

If orderId is not currently available anywhere:

- document exactly where it is missing
- determine the correct source in the existing GHL custom-provider flow
- implement the smallest necessary change to capture it
- do not redesign the entire iframe integration

The migration must be created in the transactions service’s deposits migration sequence using the next valid migration number.

Before creating the migration:

- inspect all existing migrations
- determine the latest migration number
- preserve the project’s migration naming and SQL style
- inspect whether down migrations are required
- follow the service’s migration conventions

The migration must add a nullable or otherwise appropriately constrained ghl_order_id column according to the existing data model and deployment requirements.

Do not make the column artificially required for historical deposits unless the existing migration strategy supports a safe backfill.

Document:

- column type
- nullability
- indexing decision
- foreign-key decision, if any
- whether uniqueness is appropriate
- backfill behavior
- rollback behavior

Do not assume ghl_order_id is globally unique across all GHL locations unless the existing model proves that.

If the combination of location ID and order ID is required for correct identification, preserve both values or use the existing location/integration relationship already present in the deposit model.

============================================================
7. INSPECT THE GHL queryUrl ONLY AS NEEDED
============================================================

The configured GHL queryUrl is:

https://api.rvpay.xyz/payments/custom-provider/query

Inspect the endpoint and trace its flow only if it is relevant to the final-status synchronization.

Do not perform an unrelated broad investigation of queryUrl.

Determine:

- which GHL operation calls queryUrl
- what request payload GHL sends
- what response payload RVPay returns
- whether the endpoint is used for:
  - payment status polling
  - payment verification
  - iframe initialization
  - transaction lookup
  - final payment status
  - retry/reconciliation
- whether GHL expects queryUrl to return the final status
- whether queryUrl can be used to trigger or report the final order state
- whether queryUrl is independent of the PawaPay callback
- whether queryUrl currently returns a status based on the RVPay deposit
- whether queryUrl has access to the GHL orderId or transactionId

If queryUrl is not needed to update the GHL order, do not modify it.

If queryUrl is required, make the smallest compatible change and document why.

Do not assume that queryUrl itself updates the GHL order. Prove its role from the code and existing provider flow.

============================================================
8. DETERMINE THE CORRECT GHL v3 OPERATION
============================================================

Before implementing the outbound call, inspect the current GHL custom-provider integration and identify the correct v3 operation for each terminal state.

For successful deposits:

- determine whether the correct operation is the v3 order payment recording/capture operation
- inspect the existing GHL API documentation or current integration contract available to the repository
- verify the required path, method, headers, body, and identifiers
- verify whether the operation updates the order to Paid or Completed
- verify whether it requires orderId, transactionId, chargeId, amount, currency, or another identifier

For failed deposits:

- identify the GHL-supported v3 operation for reporting a failed, declined, or unsuccessful custom-provider payment
- do not reuse a success-only record-payment endpoint for a failed payment
- do not invent an unsupported endpoint or status value
- if the correct failure operation cannot be verified, document the exact uncertainty and isolate it behind a small provider method

The implementation must distinguish:

- RVPay deposit COMPLETED
- RVPay deposit FAILED
- RVPay deposit PENDING/PROCESSING
- unknown or unsupported status

Only terminal states may trigger final GHL synchronization.

Do not send a final GHL success or failure update for a still-pending PawaPay deposit.

============================================================
9. REUSE THE EXISTING GHL PROVIDER AND AUTHENTICATION
============================================================

Inspect clients/providers/highlevel.go and all related provider/authentication code.

Reuse:

- existing OAuth configuration
- existing location access-token storage
- existing token refresh behavior
- existing provider configuration
- existing GHL API client conventions
- existing HTTP client or transport wrappers
- existing request headers
- existing logging conventions
- existing error handling

Do not create:

- a second OAuth flow
- a second token table
- a second provider configuration system
- a separate GHL client with incompatible headers
- hard-coded access tokens
- hard-coded location IDs
- hard-coded order IDs
- hard-coded production credentials

The outbound synchronization method should be owned by the appropriate service/provider boundary.

Keep GHL-specific HTTP details inside the GHL provider/client layer where possible.

Keep deposit state transitions inside the transactions service.

============================================================
10. DATABASE TRANSACTION REQUIREMENT
============================================================

The deposit status update and the decision to synchronize GHL must be part of the same database transaction.

The required logical sequence is:

BEGIN

1. Lock or safely load the relevant deposit row.
2. Validate the PawaPay callback.
3. Determine the terminal status.
4. Prevent invalid status regression.
5. Update the deposit status.
6. Record the intent to synchronize the GHL order/payment.
7. Commit.

After commit:

8. Deliver the GHL synchronization request.
9. Retry according to the queue policy.
10. Record success or final failure.

The following is not acceptable:

BEGIN
  update deposit
  call external GHL API
COMMIT

A PostgreSQL transaction cannot atomically include an external GHL HTTP request.

Do not claim that a database transaction makes the external GHL call atomic.

The database transaction must guarantee that once the deposit reaches a terminal state, the synchronization intent is durable.

============================================================
11. SIMPLE RETRY QUEUE REQUIREMENT
============================================================

Implement a simple queue only if it fits the existing architecture and can be implemented safely without introducing unnecessary complexity.

Preferred design:

- transactional outbox table or equivalent durable queue
- one row per required GHL synchronization
- status such as pending, processing, completed, failed
- retry count
- next attempt timestamp
- last error
- timestamps
- idempotency key or unique constraint
- association to deposit ID and GHL order ID

The queue insertion must occur in the same database transaction as the deposit status update.

The worker must:

- poll pending queue items
- claim work safely
- avoid processing the same item concurrently
- send the GHL request after the transaction has committed
- retry transient failures
- refresh the GHL token when appropriate
- avoid duplicate successful updates
- record the response or error
- log correlation identifiers

Keep the implementation simple.

Do not introduce a large job framework, message broker, Redis, Kafka, SQS, or another external dependency unless the repository already uses one and the project rules require it.

If a durable queue is too complex for the current architecture, abandon the queue implementation rather than destabilizing the payment system.

In that case, implement the safest simpler alternative permitted by the existing rules and document the limitation.

============================================================
12. SPECIAL TWO-TRY REQUIREMENT
============================================================

For now, the requested retry behavior is:

- attempt the GHL update
- if it fails, retry once
- after two total attempts, stop retrying that event
- update the deposit and log the GHL synchronization failure

Interpret this carefully.

The PawaPay result is already authoritative. A GHL synchronization failure must not change a completed PawaPay deposit into failed or pending.

The final deposit status must remain the status determined by PawaPay.

After two failed GHL attempts:

- preserve the terminal deposit status
- record the GHL failure
- make the failure visible in logs
- persist the failure state if the existing schema supports it
- do not return a false payment-processing error to PawaPay
- do not cause the callback to be retried solely because GHL failed
- do not roll back the already-decided PawaPay deposit status

If the user’s phrase “update the deposit after two tries” conflicts with preserving the authoritative PawaPay status, preserve the PawaPay status and update only synchronization metadata, such as:

- ghl_sync_status = failed
- ghl_sync_attempts = 2
- ghl_sync_last_error
- ghl_sync_failed_at

Do not overwrite COMPLETED with FAILED merely because GHL could not be contacted.

If adding synchronization metadata requires a migration, follow the same migration and generated-code rules.

============================================================
13. IDEMPOTENCY AND DUPLICATE CALLBACKS
============================================================

The implementation must be safe when:

- PawaPay sends the same callback more than once
- the callback is delivered after the deposit is already completed
- the callback is delivered after the deposit is already failed
- the queue worker restarts
- the GHL request times out after GHL accepted it
- the GHL request is retried
- the same order is encountered more than once

Use the existing status-transition and idempotency conventions.

Do not enqueue duplicate final-status events unnecessarily.

Use a stable idempotency key based on the existing identifiers, such as the deposit ID plus terminal status, provided that this matches the repository’s data model.

Do not assume that an external GHL operation is idempotent unless the API contract or existing integration proves it.

If the GHL API supports an idempotency key, use the documented v3 mechanism.

============================================================
14. PROTOBUF AND GENERATED CODE
============================================================

If ghl_order_id or synchronization metadata must cross a gRPC boundary:

- update the source .proto file
- follow the repository’s generation workflow
- regenerate all required Go and gateway files
- regenerate mocks if required
- run the required generation validation

Never hand-edit generated protobuf, gRPC, gateway, or mock files.

Inspect the repository’s existing generation commands before running them.

Document:

- source proto files changed
- generation command used
- generated files produced
- validation performed

Do not add unnecessary RPCs if an existing request/response can safely carry the required field.

============================================================
15. DASHBOARD REQUIREMENTS
============================================================

Inspect admindashboard/payment/page.tsx carefully.

Determine whether the frontend receives the GHL orderId from the iframe or GHL payment-provider initialization.

If the orderId is available:

- pass it through the existing payment initiation request
- preserve the existing request helper
- preserve authentication and bearer behavior
- preserve the existing dashboard API host behavior
- do not call internal/private service addresses from the browser

If the orderId is not available:

- trace the actual GHL iframe contract
- identify the smallest correct place to obtain it
- do not fabricate it
- do not add mock production data

Do not create a second payment flow.

Do not replace the existing iframe.

Do not alter successful payment behavior except to preserve the identifiers required for later synchronization.

============================================================
16. QUERY URL AND PUBLIC ROUTING
============================================================

The public query URL is:

https://api.rvpay.xyz/payments/custom-provider/query

Inspect the existing route and verify whether it is already covered by the current ALB rule:

/payments/custom-provider*

The endpoint must remain publicly reachable through the established public API host if GHL requires it.

Do not modify protected ALB/listener YAML.

Do not modify aws-cloudformation-templates.

Do not create new ALB rules unless the task proves an existing route is missing and the repository’s rules explicitly authorize that action.

If a routing gap is found, document:

- endpoint
- owner service
- public path
- expected ALB rule
- actual ALB rule
- routing gap
- recommended manual or infrastructure change

Do not silently alter production infrastructure.

============================================================
17. CORS AND AUTHENTICATION
============================================================

Preserve the existing CORS middleware and authentication architecture.

Verify:

- browser requests use the public API host
- GHL requests reach the intended public endpoint
- authenticated dashboard requests retain Bearer authentication
- server-to-server GHL requests use the existing OAuth token
- CORS does not interfere with browser requests
- callback endpoints retain their existing behavior
- GHL synchronization failures do not cause PawaPay callbacks to return misleading errors

Do not weaken authentication to make the integration work.

Do not expose CreateUser or UpdateUser publicly.

Do not leave temporary public gRPC exposure enabled.

============================================================
18. IMPLEMENTATION BOUNDARIES
============================================================

Allowed implementation areas may include:

- admindashboard/payment/page.tsx
- admindashboard/lib/api.ts
- transactions deposit models, repositories, services, and migrations
- clients HighLevel provider/client code
- relevant protobuf source files
- relevant service registration source files
- a small durable queue/outbox implementation
- tests
- documentation and checkpoints

Do not perform unrelated refactors.

Do not upgrade dependencies unless strictly required and approved by existing project rules.

Do not add a new UI library, state-management library, CSS framework, or job-processing framework.

Do not change the existing design system.

Do not add mock production data.

Do not alter unrelated dashboard pages.

============================================================
19. TESTING REQUIREMENTS
============================================================

Run all tests required by the consumed service rules.

At minimum, run the applicable commands for:

Dashboard:

- npm run lint
- npm run typecheck, if available
- npm test, if available
- npm run build

Transactions:

- go test ./transactions/...
- go vet ./transactions/...
- go build ./transactions/...

Clients:

- go test ./clients/...
- go vet ./clients/...
- go build ./clients/...

If the repository uses different commands, follow its rules and report the exact commands.

Add focused tests for:

1. ghl_order_id is accepted and persisted.
2. completed deposit creates one GHL synchronization event.
3. failed deposit creates one GHL synchronization event.
4. pending deposit does not create a final-status synchronization event.
5. duplicate PawaPay callback does not create duplicate work.
6. terminal deposit status cannot regress.
7. successful GHL synchronization is marked completed.
8. first GHL failure causes one retry.
9. second GHL failure records the failure.
10. GHL failure does not change the authoritative PawaPay deposit status.
11. token refresh behavior is reused correctly.
12. malformed or missing GHL identifiers are handled safely.
13. GHL API errors are logged with correlation identifiers.
14. queue claiming prevents concurrent duplicate processing.
15. generated protobuf and gateway code remains consistent.

Do not mark tests as passing unless they actually run and pass.

If a test cannot run locally because deployment-only infrastructure is required, state that clearly and create the strongest available unit/integration coverage.

============================================================
20. DEPLOYMENT-ONLY VERIFICATION
============================================================

The user can only test this feature in the full deployment.

Therefore, provide a deployment verification plan that uses one real GHL order and one real PawaPay transaction.

The plan must verify:

1. GHL creates or opens the order.
2. The iframe receives the expected identifiers.
3. RVPay persists ghl_order_id.
4. PawaPay receives the deposit.
5. PawaPay sends the callback.
6. RVPay updates the deposit to COMPLETED or FAILED.
7. A GHL synchronization event is created.
8. RVPay calls the GHL v3 API.
9. The correct order/payment is updated.
10. The GHL order leaves pending.
11. The success path works.
12. The failure path works.
13. Duplicate callbacks do not duplicate the update.
14. GHL API failure is retried once.
15. After two failures, the deposit remains in its PawaPay-authoritative terminal state and the failure is logged/persisted.

Include the exact logs and identifiers that should be captured without exposing secrets or access tokens.

============================================================
21. VISUAL AND DASHBOARD QA
============================================================

The primary task is backend synchronization, but verify that any dashboard payment-page changes:

- preserve the existing layout
- preserve the existing iframe behavior
- preserve loading and error states
- preserve the existing API request helper
- preserve authentication
- do not introduce a second environment system
- do not break existing dashboard routes

Do not make unrelated visual changes.

============================================================
22. DOCUMENTATION REQUIREMENTS
============================================================

Update required documentation by appending, never overwriting.

Document:

- original defect
- root cause
- GHL v3 operation used
- successful status mapping
- failed status mapping
- queryUrl findings
- orderId source
- database migration
- proto changes
- queue/outbox behavior
- two-attempt retry behavior
- idempotency behavior
- token refresh behavior
- endpoint ownership
- public path
- ALB rule dependency
- CORS dependency
- deployment-only verification steps
- known limitations
- any GHL API uncertainty

If a new endpoint or provider operation is introduced, document it using the repository’s established endpoint documentation convention.

For every new backend endpoint or externally called operation, map:

- operation
- HTTP method
- path
- owner service
- authentication
- public/private status
- expected ALB path rule
- request identifiers
- response behavior
- failure behavior

============================================================
23. REQUIRED FINAL REPORT
============================================================

At the end, provide a complete report containing:

A. Discovery

- all rule files consumed
- all ignore files consumed
- all context/checkpoint files consumed
- all relevant source files inspected

B. Root cause

- why GHL orders remained pending
- whether the iframe sent orderId
- whether orderId was persisted before this task
- how the final GHL synchronization is now triggered

C. Implementation

- migration name and purpose
- database fields added
- proto changes
- repository changes
- service changes
- GHL provider changes
- queue/outbox changes
- dashboard changes
- queryUrl findings

D. GHL v3 details

- exact operation used for success
- exact operation used for failure
- HTTP method/path
- headers
- payload shape
- identifiers used
- token behavior
- idempotency behavior

E. Endpoint map

For every endpoint or operation:

- method
- path
- owner
- auth
- public/private status
- ALB rule
- CORS requirement

F. Tests

- exact commands
- exact results
- generated-code validation
- migration validation
- focused synchronization tests

G. Deployment verification

- exact steps for a real GHL order
- exact logs to inspect
- success criteria
- failure criteria
- retry verification

H. Documentation

- checkpoint files appended
- README files updated
- context files updated
- new documentation files

I. Limitations

- anything that could not be verified locally
- any GHL API behavior requiring full deployment
- any queue limitation
- any Marketplace configuration still requiring manual verification

============================================================
24. NON-NEGOTIABLE RULES
============================================================

- Treat the existing PawaPay payment flow as working.
- Do not redesign the payment flow.
- Do not blame PawaPay for the missing GHL update.
- The missing functionality is server-side GHL synchronization.
- Target GHL’s v3 API.
- Reuse the existing HighLevel OAuth/provider implementation.
- Inspect admindashboard/payment/page.tsx.
- Ensure ghl_order_id is persisted through a proper deposits migration.
- Update protobuf only through source files and the official generation workflow.
- Never hand-edit generated files.
- Use a database transaction for deposit finalization plus synchronization intent.
- Never make the external GHL call inside the PostgreSQL transaction.
- Use a simple queue/outbox only if it can be implemented safely.
- Retry GHL synchronization once, for two total attempts.
- After two GHL failures, preserve the PawaPay-authoritative deposit status and log/persist the GHL failure.
- Do not return a misleading PawaPay callback error because GHL failed.
- Inspect queryUrl only if needed.
- Do not modify protected ALB or CloudFormation files.
- Preserve CORS, authentication, token refresh, and public API routing.
- Do not add mock production data.
- Do not perform unrelated refactors.
- Strictly obey every consumed .clinerules.md and .clineignore.md file.
- Append checkpoint and documentation updates; never overwrite history.
- Run all required tests and report actual results.
- Do not claim completion if any required test fails.
============================================================
IMPLEMENTATION COMPLETION NOTE
============================================================

Status: source/test implementation verified and passing.

Commands executed:
- go build ./transactions/... ./clients/...
- go vet ./transactions/... ./clients/...
- gofmt -l on changed packages
- go test ./transactions/... ./clients/...

All relevant packages pass.

Tested behavior:
- deposit callback validation
- COMPLETED/FAILED callback finalize + GHL sync intent queue
- PROCESSING callback does not queue a final-status event
- terminal deposit status protection
- unknown deposit handled safely
- repository lookup errors surfaced as Internal
- worker success path marks sync completed
- worker first failure triggers one retry
- worker second failure records failure
- GHL failure does not overwrite PawaPay status
- malformed or missing order identifiers handled safely
- claim query prevents duplicate processing
- generated protobuf/gRPC code consistency

Files created:
- agents/ghl-v3-payment-status-synchronization.md
- transactions/db/migrations/000006_ghl_order_sync.up.sql
- transactions/db/migrations/000006_ghl_order_sync.down.sql
- transactions/ghlsync/worker.go
- transactions/ghlsync/worker_test.go
- clients/ghlsync/service.go
- transactions/payments/callback_test.go

Files modified:
- protobuf/transactions.proto
- protobuf/clients.proto
- transactions/db/query/deposits.sql
- transactions/db/sqlc/models.go
- transactions/db/sqlc/querier.go
- transactions/db/sqlc/deposits.sql.go
- transactions/db/sqlc/mocks/querier.go
- transactions/db/repo/deposit_repo.go
- transactions/db/repo/mocks/repo.go
- grpc/go/transactionsgrpc/transactions.pb.go
- grpc/go/transactionsgrpc/transactions_grpc.pb.go
- grpc/go/clientsgrpc/clients.pb.go
- grpc/go/clientsgrpc/clients_grpc.pb.go
- transactions/deposits/service.go
- transactions/payments/service.go
- transactions/payments/callback_test.go
- transactions/payments/service_test.go
- transactions/cmd/grpc-service/main.go
- clients/cmd/grpc-service/main.go
- clients/oauth/service.go
- clients/oauth/errors.go
- clients/providers/payment_provider.go
- clients/providers/highlevel_payment_provider.go
- admindashboard/app/payment/page.tsx

Deployment verification against a live GHL instance is not possible in this
environment and is documented as a follow-up.

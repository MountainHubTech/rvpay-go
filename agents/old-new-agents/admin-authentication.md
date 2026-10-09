# Cline Agent Task — Minimal Secure Admin Authentication

## Mission

Implement a **minimal but strongly secured administrator authentication flow** for RVPay.

The primary areas of work are:

```text
admindashboard/
clients/
```

You are also explicitly permitted to modify:

```text
transactions/
```

but ONLY where necessary to protect Dashboard/admin endpoints with the new access-token authentication middleware.

The authentication system must integrate into the project's **existing architecture and coding conventions**.

Do NOT introduce a new architectural style, framework, authentication library, API convention, naming convention, or syntax merely because it is personally preferred.

Before changing anything, inspect the existing project carefully and follow the patterns already established in the repository.

---

# 1. NON-NEGOTIABLE BOUNDARIES

## You MUST

* Read the relevant README/project documentation first.
* Read the applicable `.clinecheck.md` files.
* Read the applicable `.service-checkpoint.md` files.
* Read the applicable `.clineignore.md` files.
* Read the applicable `.project-checkpoint.md` files.
* Read the applicable `.clinerules.md` files.
* Adhere strictly to the `.clinerules.md` and `.clineignore` directives.
* Read the existing project checkpoint/context documentation.
* Inspect existing protobuf definitions before modifying API contracts.
* Inspect generated gateway code before determining how routes are wired.
* Inspect existing service/repository/database patterns.
* Inspect existing migrations/schema conventions.
* Inspect existing middleware patterns.
* Inspect existing logging/observability patterns.
* Inspect existing Dashboard API/environment architecture.
* Inspect existing Dashboard components before creating new UI components.
* Preserve existing naming and syntax conventions.
* Use the project's existing protobuf generation process.
* Never manually edit generated protobuf/gateway files if the repository has an established generation workflow.
* Append to the appropriate checkpoint/context documents when complete.
* Run the appropriate backend and Dashboard tests/builds.
* Report exactly what was changed and why.

## You MUST NOT

Do NOT:

* redesign the application's architecture;
* introduce OAuth;
* introduce JWT unless the existing project already uses JWT and inspection proves it is appropriate;
* introduce sessions/cookies as a replacement for the requested token model;
* introduce signup;
* introduce password-reset functionality;
* introduce email verification;
* introduce MFA;
* introduce social login;
* introduce an authentication provider;
* introduce a new database;
* introduce Redis merely for authentication;
* expose passwords;
* expose refresh tokens;
* expose access tokens in logs;
* log Authorization headers;
* log request bodies containing credentials;
* modify ALB listener rules;
* modify ALB YAML;
* modify deployment infrastructure;
* change existing environment routing;
* modify payment behavior except where explicitly required to keep the payment page/InitiateDeposit exempt from admin authentication.

Keep this implementation deliberately small.

---

# 2. FIRST: UNDERSTAND THE EXISTING CODEBASE

Before writing code, inspect at minimum:

```text
README.md

clients/
clients/.clinecheck.md
clients/.service-checkpoint.md
clients/.clinerules.md
clients/.clineignore.md

transactions/
transactions/.clinecheck.md
transactions/.service-checkpoint.md
transactions/.clinerules.md
transactions/.clineignore.md

admindashboard/
admindashboard/.clinecheck.md
admindashboard/.project-checkpoint.md
admindashboard/.dashboard-setup.md
admindashboard/.clinerules.md
admindashboard/.clineignore.md
```

Also inspect any repository-level documentation that the existing agents have been instructed to read.

Then inspect:

### Backend

```text
clients/config/
clients/cmd/
clients/service/
clients/repository/
clients/proto/
clients/migrations/
```

Use the actual repository structure if these paths differ.

Also inspect:

```text
transactions/cmd/
transactions/service/
transactions/proto/
transactions/migrations/
```

### Dashboard

Inspect:

```text
admindashboard/lib/
admindashboard/app/
admindashboard/components/
```

especially:

```text
admindashboard/lib/api.ts
admindashboard/app/settings/
admindashboard/app/
admindashboard/components/dashboard/
admindashboard/components/
```

Find the existing:

* API client;
* environment selector;
* sidebar;
* user section;
* layout;
* shared input components;
* buttons;
* cards;
* typography;
* error handling;
* localStorage/context patterns.

Do not create duplicates of existing functionality.

---

# 3. Authentication Model

The system has two token types:

```text
Refresh Token
Access Token
```

The database is the source of truth.

The authentication system belongs to the **Clients service**.

---

# 4. Users Table

Create a database table:

```text
users
```

with the following logical fields:

```text
id
name
email
password
user_role
refresh_token
```

Use the repository's existing conventions for:

* primary keys;
* UUIDs;
* timestamps, if existing tables consistently use them;
* column naming;
* migrations;
* constraints;
* indexes.

Do not invent a completely different schema style.

---

# 5. USER_ROLE Enum

Introduce:

```text
USER_ROLE_USER
USER_ROLE_ADMIN
```

using the project's existing enum conventions.

The exact database representation must follow the project's established PostgreSQL/protobuf conventions.

The application must distinguish:

```text
USER_ROLE_USER
```

from:

```text
USER_ROLE_ADMIN
```

Authentication alone is NOT sufficient for an admin endpoint.

Admin endpoints require the authenticated user to have:

```text
USER_ROLE_ADMIN
```

---

# 6. User Email

The email must uniquely identify a user.

Enforce uniqueness at the database level.

Do not rely only on application-level checking.

Email comparison should follow the existing project conventions.

Do not silently introduce complicated email normalization logic unless the repository already has an established convention.

---

# 7. Password Security

Passwords MUST NEVER be stored in plaintext.

Use a modern password hashing algorithm already available/appropriate for the Go project.

Preferred approach:

```text
Argon2id
```

if introducing it fits the project's dependency policy.

If the existing project already uses a strong password hashing implementation, follow that existing implementation instead.

Do NOT use:

```text
MD5
SHA-1
SHA-256 alone
plain SHA hashing
```

for password storage.

The password hashing implementation must include a cryptographically secure salt.

The stored password should therefore be the password-hash representation, not:

```text
password + salt
```

in separate plaintext columns.

If the selected password hashing format embeds the salt into the encoded hash, use that standard representation.

Password verification must use a constant-time-safe verification implementation appropriate to the selected algorithm.

---

# 8. Admin Account Creation

There is NO public signup endpoint.

The initial admin account is created by the backend.

Inspect the existing project's preferred mechanism for bootstrapping required database records.

Prefer an existing migration/seed/bootstrap mechanism over creating an arbitrary new mechanism.

The initial administrator credentials MUST NOT be hard-coded into application source code.

Do not put a real password in:

```text
Go source
proto files
Dashboard source
README
checkpoint files
logs
tests
```

If the repository already has an established secure environment/seed mechanism, use it.

If no suitable mechanism exists, implement the smallest backend-only bootstrap mechanism consistent with the existing configuration conventions and document exactly how the operator supplies the initial admin credentials.

The initial admin must have:

```text
USER_ROLE_ADMIN
```

---

# 9. Refresh Token

Each user has:

```text
refresh_token
```

stored in the `users` table.

Refresh tokens must be cryptographically random and extremely difficult to guess.

Do NOT derive refresh tokens from:

* user IDs;
* emails;
* passwords;
* timestamps;
* predictable UUID sequences;
* hashes of predictable values.

Use a cryptographically secure random generator.

The raw refresh token must NEVER be logged.

---

# 10. Access Tokens Table

Create:

```text
access_tokens
```

with at minimum:

```text
user_id
refresh_token
access_token
```

Also add whatever metadata is necessary for secure expiration handling according to the existing schema conventions.

At minimum, the system needs to know when an access token expires.

Prefer an explicit expiration field such as:

```text
expires_at
```

rather than attempting to infer expiration from token creation time elsewhere.

Use appropriate indexes and foreign-key relationships consistent with the existing database style.

---

# 11. Access Token Security

Access tokens must also be cryptographically random and unguessable.

Do NOT use:

```text
user ID
email
timestamp
incrementing value
predictable UUID
```

as an access token.

Use a cryptographically secure random token generator.

Access tokens must have an expiration of exactly:

```text
3 hours
```

from creation.

The raw access token must never be written to logs.

Do not include access tokens in:

* request logs;
* debug logs;
* error logs;
* database query logs;
* checkpoint files;
* test output;
* Dashboard console logging.

---

# 12. IMPORTANT TOKEN STORAGE SECURITY

The specification requires the tokens to exist in the database.

Do not interpret that as permission to expose them unnecessarily.

Inspect whether the project's database access layer can support storing a **hash of the token** while returning the raw token only once to the client.

If this can be implemented cleanly within the existing architecture, strongly prefer:

```text
raw token
    ↓
returned to client
    ↓
cryptographic hash stored in database
```

rather than storing reusable bearer credentials in plaintext.

However, if the existing project's architecture or stated contract explicitly requires the raw token to be stored, follow that requirement and ensure:

* it is never logged;
* it is never returned through normal user/database APIs;
* it is only accessed by authentication code;
* database access is restricted through the existing repository/service boundaries.

Do not make a large security architecture change solely to introduce token hashing.

Document the chosen approach.

---

# 13. Sign-In Flow

Create an authentication endpoint in the Clients service.

Use the existing API naming and protobuf conventions.

Conceptually:

```text
POST /v1/public/auth/sign-in
```

or the equivalent route that fits the project's existing public API conventions.

The exact route MUST be determined by inspecting the existing protobuf/API organization.

Sign-in is public because the user is not authenticated yet.

The request contains:

```text
email
password
```

The service:

1. Finds the user by email.
2. Verifies the password hash.
3. Confirms the account is active according to the project's user model, if such a concept exists.
4. Generates a cryptographically random refresh token if required.
5. Generates a cryptographically random access token.
6. Sets the access token expiration to three hours.
7. Persists the required token state.
8. Returns the access token to the Dashboard.
9. Returns only the minimum information required by the Dashboard.

Do NOT return:

```text
password
password hash
refresh token
internal database details
```

unless the specified refresh flow explicitly requires the refresh token to be returned.

---

# 14. Refresh Flow

Create the authentication endpoint required to exchange a valid refresh token for a new access token.

Conceptually:

```text
POST /v1/public/auth/refresh
```

Use the repository's existing naming/protobuf conventions.

The refresh request must contain the refresh token.

The service must:

1. Validate the refresh token.
2. Identify the user.
3. Verify the user still exists and is eligible to authenticate.
4. Generate a new cryptographically random access token.
5. Set its expiration to three hours.
6. Persist the new access token.
7. Only AFTER successful database persistence return the new access token.

---

# 15. CRITICAL TRANSACTION REQUIREMENT

The refresh flow MUST use a database transaction.

The logical operation is:

```text
BEGIN

validate refresh token

generate access token

persist access token

COMMIT

return access token
```

The response containing the new access token MUST NOT be sent if the database transaction fails.

If persistence fails:

```text
ROLLBACK
```

and return an appropriate authentication/server error.

Never:

```text
generate token
send token
attempt database insert
```

That violates the required guarantee.

The database operation must succeed before the token becomes usable by the client.

---

# 16. Access Token Validation

Introduce authorization middleware.

The middleware must:

1. Read the HTTP `Authorization` header.
2. Require:

```text
Authorization: Bearer <access-token>
```

3. Reject missing authorization.
4. Reject malformed authorization.
5. Extract the bearer token.
6. Validate the token against the authentication data in the Clients service/database according to the chosen storage strategy.
7. Verify that the token has not expired.
8. Resolve the associated user.
9. Verify the user's role when the route requires admin access.
10. Allow the request to continue only when validation succeeds.

Do not log the Authorization header.

Do not log the token.

---

# 17. Middleware Placement

The authorization middleware must sit in front of admin endpoints.

Conceptually:

```text
HTTP request
    ↓
CORS
    ↓
Access Log
    ↓
Authorization
    ↓
grpc-gateway
    ↓
service
    ↓
repository
```

However, use the project's existing middleware ordering conventions.

The important requirement is:

```text
unauthorized request
    ↓
MUST NOT reach the protected handler/business logic
```

Do not put authorization inside individual business methods when a transport middleware can enforce it consistently.

---

# 18. Cross-Service Authorization Architecture

The Dashboard has endpoints in both:

```text
clients
transactions
```

but the authentication data lives in:

```text
clients
```

Do NOT duplicate the users table into Transactions.

Do NOT create a second authentication system in Transactions.

Determine the smallest architecture that allows Transactions admin endpoints to validate the access token against the Clients authentication authority while respecting the existing service architecture.

Inspect the existing gRPC communication patterns.

Prefer an internal service-to-service authentication/authorization mechanism that fits the existing architecture.

Do not introduce a new external authentication service.

Do not make the Dashboard responsible for validating tokens.

The backend remains responsible for authentication.

---

# 19. Admin Role Enforcement

All protected Dashboard endpoints must require:

```text
USER_ROLE_ADMIN
```

A valid token belonging to:

```text
USER_ROLE_USER
```

must NOT be sufficient.

The authorization decision is:

```text
valid access token
+
user exists
+
token not expired
+
user role == USER_ROLE_ADMIN
=
ALLOW
```

Anything else:

```text
DENY
```

Return appropriate HTTP/gRPC authentication/authorization status codes according to the existing project conventions.

Use:

```text
401
```

for missing/invalid authentication where appropriate.

Use:

```text
403
```

when the user is authenticated but lacks the required admin role.

Do not leak whether an arbitrary email exists during failed sign-in attempts.

---

# 20. Protect Dashboard Endpoints

All existing Dashboard endpoints must move from:

```text
/v1/public/...
```

to:

```text
/v1/admin/...
```

except:

1. `InitiateDeposit`
2. the payments page's explicitly required public/payment endpoints.

This means the Dashboard's administrative data APIs must no longer be public.

Use the route structure established by the previous ALB-compatible routing work.

For example, if an existing endpoint is:

```text
/v1/public/transactions/overview/snapshot
```

the protected equivalent should become:

```text
/v1/admin/transactions/overview/snapshot
```

If an existing endpoint is:

```text
/v1/public/clients/sub-accounts
```

the protected equivalent should become:

```text
/v1/admin/clients/sub-accounts
```

The exact final paths must be derived from the current repository.

---

# 21. IMPORTANT ALB CONSTRAINT

The existing ALB listener rules remain authoritative.

DO NOT modify ALB YAML or listener rules.

Because the existing ALB rules permit prefixes such as:

```text
/v1/public/clients*
/v1/public/payouts*
/v1/public/transactions*
```

inspect the current infrastructure/routing carefully before converting:

```text
/v1/public
```

to:

```text
/v1/admin
```

The new `/v1/admin/...` routes must still be reachable through the existing infrastructure.

If the existing ALB rules do not permit the proposed `/v1/admin/...` path, DO NOT modify the ALB.

Instead, STOP and report the infrastructure conflict.

However, before declaring a conflict, inspect the existing route architecture carefully to determine whether the admin endpoint can retain an already-permitted prefix while still being logically protected.

The user's explicit requirement is:

> Admin endpoints should conceptually move from `/v1/public` to `/v1/admin`.

Do not silently change the ALB configuration to accomplish this.

---

# 22. Dashboard Sign-In Page

Add a sign-in page to:

```text
admindashboard/
```

Use the existing Dashboard design system.

The page should contain:

```text
Email
Password
Sign In
```

centered on the screen.

Use existing:

* input components;
* button components;
* typography;
* spacing;
* cards;
* layout primitives;
* existing colors.

DO NOT introduce new colors.

DO NOT create a new visual design language.

Keep it simple and consistent with the existing Dashboard.

---

# 23. No Signup

There must be NO:

```text
Sign Up
Register
Create Account
Forgot Password
Reset Password
```

flow.

The admin account is backend-created.

The sign-in page only authenticates an existing account.

---

# 24. Authentication Context

Create an authentication context/provider using the Dashboard's existing React architecture.

The context should hold the currently authenticated user's authentication state.

At minimum it needs to know:

```text
authenticated / unauthenticated
access token
admin name
admin email
```

Do not expose unnecessary user information.

---

# 25. Access Token Lifetime in Dashboard

The access token is valid for:

```text
3 hours
```

The Dashboard must respect this expiration.

Do not silently keep using an expired token.

When the access token expires:

```text
Dashboard request
    ↓
token invalid/expired
    ↓
user must authenticate again
```

The user should ultimately be returned to the sign-in page.

Do NOT automatically create a new access token without validating the refresh token through the backend refresh flow.

---

# 26. Protecting the Access Token in the Dashboard

Treat the access token as sensitive.

Do NOT:

* log it;
* display it;
* put it in URLs;
* put it in query strings;
* expose it in error messages;
* place it in normal Dashboard UI state unnecessarily;
* send it to unrelated APIs.

Use the safest storage mechanism compatible with the existing Dashboard architecture.

Prefer keeping the token in memory rather than persistent browser storage if practical.

If persistence across a page reload is required, carefully inspect the requested refresh-token architecture and existing application constraints before choosing storage.

Do not automatically put the access token into `localStorage` merely because the Dashboard already uses localStorage for the environment selector.

The environment selector and authentication credentials are fundamentally different security concerns.

---

# 27. Refresh Token in Dashboard

The refresh token is more sensitive than the access token.

Do NOT expose the refresh token to normal Dashboard components.

Do NOT put it in:

```text
URL
query string
console logs
React props
visible UI
```

Prefer a secure HttpOnly cookie architecture if the existing application/API architecture supports it without a major redesign.

If that is incompatible with the current architecture, inspect the existing constraints and choose the smallest secure implementation possible.

Document the decision.

---

# 28. API Client

Update:

```text
admindashboard/lib/api.ts
```

so protected requests automatically attach:

```text
Authorization: Bearer <access-token>
```

Do not duplicate token-handling code throughout individual pages.

Centralize it in the existing API abstraction.

The API client must continue respecting the existing:

```text
Local
Testing
Production
```

environment selection.

The environment selector must NOT contain authentication logic.

---

# 29. Public Requests

The API client must distinguish between:

```text
public authentication/payment requests
```

and:

```text
admin-protected requests
```

For example:

```text
Sign In
    → public

Refresh
    → public authentication endpoint

InitiateDeposit
    → public/payment exception

Payment page
    → existing required public behavior

Dashboard overview
    → admin

Sub-accounts
    → admin

Payouts
    → admin

Transactions
    → admin
```

Determine the exact endpoint inventory by inspecting the current Dashboard.

---

# 30. Route Protection in Next.js

No one should be able to access the Dashboard UI without authentication.

Inspect the existing Next.js architecture/version and use its established routing conventions.

Implement an authentication guard at the appropriate application boundary.

Unauthenticated users attempting to access:

```text
/
 /sub-accounts
 /transactions
 /payouts
 /settings
```

and other protected Dashboard pages must be redirected to:

```text
/sign-in
```

Do not create route guards independently in every page if a centralized mechanism is available.

The sign-in page itself must remain accessible without authentication.

The payment page must retain its required public behavior.

---

# 31. Sidebar User Section

The Dashboard sidebar currently contains a user section at the bottom with hard-coded values.

Replace those values with the authenticated admin's:

```text
name
email
```

from the authentication state.

Do not hard-code:

```text
Admin
admin@example.com
```

or similar values.

---

# 32. Sign Out

Make the existing Sign Out button functional.

On sign out:

1. Clear the Dashboard's authentication state.
2. Remove the access token from memory/storage.
3. Invalidate the server-side authentication state where the implemented token model supports it.
4. Do not leave a reusable credential behind.
5. Redirect the user to:

```text
/sign-in
```

6. Protected Dashboard pages must no longer be accessible.

Do not merely navigate to `/sign-in` while leaving the authentication state intact.

---

# 33. Token Revocation / Logout

Because the database is the source of truth for authentication, determine the smallest safe way to invalidate the current authentication state during logout.

Do not introduce a complicated token-revocation subsystem.

At minimum, ensure a signed-out user cannot simply return to the previous Dashboard page and continue making authenticated requests with a still-valid token if the existing architecture permits server-side invalidation.

Document exactly what happens to:

```text
access_tokens
refresh_token
```

on sign out.

---

# 34. Error Handling

Authentication errors must be generic enough not to leak sensitive information.

For sign-in failure, do NOT reveal:

```text
email exists
email does not exist
password was wrong
```

as separate externally visible conditions.

Use an appropriate generic authentication failure.

For protected endpoints:

```text
missing token → 401
invalid token → 401
expired token → 401
valid user without admin role → 403
```

where consistent with the existing API/error conventions.

---

# 35. Logging

Use the existing zerolog/observability conventions.

Authentication events may be logged at a useful level, but NEVER log:

```text
password
password hash
refresh token
access token
Authorization header
```

Safe fields may include:

```text
operation
request_id
user_id
user_role
endpoint
method
status
duration
```

Use the existing project's terminology and logging structure rather than inventing a new logging format.

---

# 36. Brute-Force Consideration

Keep the implementation minimal.

Do not introduce a large rate-limiting framework.

However, inspect the existing HTTP middleware infrastructure and determine whether there is already a suitable mechanism for limiting repeated authentication attempts.

If there is no existing mechanism, do not build a complicated distributed rate limiter as part of this task.

Document that brute-force/rate limiting is a future hardening item if necessary.

Password hashing itself must nevertheless use an appropriately expensive modern password hashing algorithm.

---

# 37. Database Transactions

The following operations must use transactions where atomicity is required.

Especially:

```text
refresh token validation
+
new access token creation
```

The new access token MUST NOT be returned before its database transaction successfully commits.

Follow existing repository transaction patterns.

Do not introduce a new transaction abstraction if one already exists.

---

# 38. Protobuf / API Contract

Inspect the existing protobuf structure before adding:

```text
SignIn
Refresh
SignOut
```

Use the project's established:

* package naming;
* service naming;
* request/response naming;
* HTTP annotations;
* validation;
* generated gateway workflow.

Do not invent a second API style.

Regenerate generated code using the project's existing Makefile/scripts.

---

# 39. Admin Endpoint Protection in Transactions

Transactions must not independently invent its own user/password authentication.

The Transactions service should validate the access token using the established authentication authority created for the Clients service.

Determine the cleanest existing service-to-service pattern.

The Dashboard should not have to understand which service owns the user database.

The desired conceptual flow is:

```text
Dashboard
   │
   │ Authorization: Bearer <access-token>
   ▼
Transactions HTTP middleware
   │
   │ validate token/admin identity
   ▼
Transactions protected endpoint
```

and:

```text
Dashboard
   │
   │ Authorization: Bearer <access-token>
   ▼
Clients HTTP middleware
   │
   │ validate token/admin identity
   ▼
Clients protected endpoint
```

---

# 40. Existing CORS

Preserve the existing shared CORS implementation.

Do not remove it.

Do not duplicate it.

Do not create authentication-specific CORS handling.

The middleware chain should continue to allow the Dashboard origin before authentication processing.

For protected endpoints, the logical order should remain compatible with:

```text
CORS
→ logging
→ authorization
→ gateway
→ service
```

Use the existing implementation's actual ordering conventions.

---

# 41. ALB / Route Compatibility

This task must respect the routing work already completed.

Before changing routes, inspect the current Dashboard API paths.

The resulting routes must satisfy both:

```text
authentication semantics
```

and:

```text
existing ALB routing constraints
```

Never solve a route mismatch by modifying the ALB.

If a required `/v1/admin/...` endpoint cannot pass through the existing ALB rules, document the exact conflict rather than changing infrastructure.

---

# 42. Testing Requirements

Add tests for password hashing.

At minimum:

```text
same password → successful verification
wrong password → failed verification
stored value != plaintext password
salted/hashed representation is not deterministic plaintext
```

Add authentication tests for:

```text
successful sign-in
invalid email/password
successful token creation
expired access token
invalid access token
missing Authorization header
malformed Authorization header
USER_ROLE_USER rejected from admin endpoint
USER_ROLE_ADMIN accepted
```

Add refresh tests for:

```text
valid refresh token → new access token
invalid refresh token → rejection
expired/invalid user state → rejection
database failure → no token returned
transaction rollback → no unusable authentication state
```

Add middleware tests proving protected handlers are NOT called when authorization fails.

Add Dashboard tests where the existing testing framework supports them for:

```text
unauthenticated → /sign-in
authenticated admin → Dashboard
sign out → /sign-in
admin name/email displayed
protected API requests carry Authorization header
```

Do not create an elaborate testing framework just for this task.

Use existing project testing patterns.

---

# 43. Security Test Requirement

Explicitly verify that no authentication secrets appear in logs.

Search the changed code for logging of:

```text
password
refresh_token
access_token
Authorization
Bearer
```

and make sure secrets are not being logged.

Do not treat a test that prints a token as acceptable merely because it is a test.

---

# 44. Dashboard UI Requirements

The sign-in page should be visually simple.

Use existing components.

Conceptually:

```text
┌───────────────────────────────┐
│                               │
│           Sign In             │
│                               │
│  Email                        │
│  [_________________________]  │
│                               │
│  Password                     │
│  [_________________________]  │
│                               │
│       [     Sign In     ]     │
│                               │
└───────────────────────────────┘
```

Do not introduce:

* new colors;
* gradients;
* animations;
* decorative graphics;
* new icon libraries;
* unrelated UI dependencies.

Use what already exists.

---

# 45. Loading / Error States

The sign-in form must:

* prevent accidental duplicate submission;
* show an existing-style loading state;
* display an appropriate existing-style error state;
* clear/handle errors appropriately after another attempt.

Do not display raw backend errors containing internal information.

---

# 46. Session Initialization

On Dashboard startup, determine authentication state before allowing protected pages to render.

Avoid a flash where:

```text
unauthenticated user
```

briefly sees protected Dashboard content before being redirected.

Use the existing React/Next.js architecture to provide a clean loading/authentication initialization state.

Do not introduce a large state-management library.

---

# 47. Access Token Expiration

The Dashboard should understand the three-hour access-token lifetime.

If the token is expired locally, do not repeatedly send known-expired tokens.

The application should transition to the unauthenticated state and redirect to sign-in.

If the backend responds with:

```text
401
```

because the access token is expired/invalid, the Dashboard should clear authentication state and redirect appropriately.

Do not implement silent refresh unless it is explicitly required by the final architecture.

The requirement is that after expiration the user must validate the refresh token to obtain a new access token; do not bypass that requirement with client-side token fabrication or indefinite session extension.

---

# 48. Refresh Token Validation

The refresh token must be validated by the backend.

The Dashboard must never decide that a refresh token is valid.

The backend remains authoritative.

The Dashboard must never generate:

```text
access tokens
refresh tokens
password hashes
```

itself.

---

# 49. Do Not Break Existing Environment Switching

Preserve:

```text
Local
Testing
Production
```

and the existing base URLs:

```text
Local:
Clients       http://localhost:8080
Transactions  http://localhost:8081

Testing:
Clients       https://api.rvpay.xyz
Transactions  https://api.rvpay.xyz

Production:
Clients       https://api.rvpay.co
Transactions  https://api.rvpay.co
```

Authentication requests must use the currently selected environment.

Do not make authentication permanently point to Testing or Production.

---

# 50. Important Security Boundary

Never trust the Dashboard to declare:

```text
I am an admin
```

The Dashboard UI is untrusted.

Admin status must come from the backend's authenticated user record:

```text
users.user_role
```

The backend must enforce admin authorization independently of the UI.

Hiding a button or route in React is NOT authorization.

The server middleware must reject unauthorized requests.

---

# 51. Migration Safety

Use the existing migration system.

Do not modify existing production data destructively.

Do not:

```text
DROP TABLE
DROP DATABASE
DELETE users
```

as part of normal migration.

If existing database conventions require a migration version, follow them.

---

# 52. Dependency Discipline

Before adding a new Go dependency or npm dependency:

1. Check whether the repository already has an equivalent.
2. Prefer the standard library where it provides an appropriately secure implementation.
3. Otherwise use a well-maintained dependency appropriate for production.
4. Do not add multiple libraries that solve the same problem.

Do not introduce a framework solely for authentication.

---

# 53. Code Style

This is extremely important.

Before implementing anything:

```text
READ THE EXISTING CODE.
```

Do not write code based on generic examples from the internet.

Match the repository's existing:

* function naming;
* receiver style;
* constructor style;
* error handling;
* context usage;
* logging;
* repository interfaces;
* database transaction handling;
* protobuf style;
* middleware style;
* React component style;
* TypeScript conventions;
* import ordering;
* formatting;
* file organization.

Do not introduce syntax that does not already belong naturally in this codebase.

---

# 54. Generated Code

If protobuf changes are required:

```text
modify .proto
    ↓
run existing generation command
    ↓
inspect generated changes
    ↓
test
```

Never hand-edit generated:

```text
.pb.go
_grpc.pb.go
.pb.gw.go
```

files when the repository has an established generator.

---

# 55. Documentation

At completion, APPEND to the relevant existing documentation.

Do NOT overwrite existing checkpoint/history.

At minimum inspect and append to the applicable:

```text
clients/.clinecheck.md
clients/.service-checkpoint.md

transactions/.clinecheck.md
transactions/.service-checkpoint.md

admindashboard/.clinecheck.md
admindashboard/.project-checkpoint.md
```

Only update files that actually exist/apply.

Document:

* authentication architecture;
* users schema;
* access_tokens schema;
* password hashing approach;
* token generation/storage approach;
* access-token lifetime;
* refresh flow;
* admin middleware;
* protected routes;
* public exceptions;
* Dashboard auth context;
* sign-in/sign-out flow;
* environment interaction;
* tests;
* known limitations/future hardening.

APPEND; do not rewrite historical entries.

---

# 56. Final Route Inventory

Before declaring completion, produce a complete table:

```text
Endpoint
HTTP Method
Service
Old Route
New Route
Public/Admin
Authorization Required
ALB-Compatible Prefix
Dashboard Caller
```

Every existing Dashboard endpoint must appear.

This is especially important because the previous routing work moved Dashboard endpoints to fit the existing ALB prefixes.

---

# 57. Final Security Checklist

Before finishing, verify:

```text
[ ] Passwords are never stored plaintext
[ ] Password hashing uses a strong salted algorithm
[ ] Password hashes never appear in responses
[ ] Refresh tokens are cryptographically random
[ ] Access tokens are cryptographically random
[ ] Access tokens expire after exactly 3 hours
[ ] Tokens are never logged
[ ] Authorization headers are never logged
[ ] Sign-in does not reveal whether an email exists
[ ] Refresh uses a DB transaction
[ ] Access token is not returned before DB commit
[ ] Invalid tokens are rejected
[ ] Expired tokens are rejected
[ ] USER_ROLE_USER cannot access admin endpoints
[ ] USER_ROLE_ADMIN can access admin endpoints
[ ] Admin middleware executes before protected business logic
[ ] Dashboard cannot render protected pages while unauthenticated
[ ] Sign out actually clears authentication
[ ] Sign out redirects to /sign-in
[ ] No signup exists
[ ] Payment exception remains functional
[ ] InitiateDeposit remains functional
[ ] CORS remains functional
[ ] ALB YAML was NOT changed
[ ] No new authentication framework was introduced unnecessarily
```

---

# 58. Validation

Run all relevant backend validation:

```bash
go build ./...
go vet ./clients/... ./transactions/... ./shared/...
go test ./clients/... ./transactions/... ./shared/... -count=1
```

Run the Dashboard validation:

```bash
npm run build
npm run lint
```

Also run the repository's existing protobuf generation/check commands.

If there are pre-existing failures, clearly distinguish:

```text
pre-existing
```

from:

```text
introduced by this task
```

Do not hide failures.

---

# 59. Manual Verification

Where possible, verify the following manually.

## Sign in

```text
Dashboard → /sign-in
email + password
→ successful authentication
→ Dashboard
```

## Invalid credentials

```text
wrong password
→ generic authentication error
→ remain on /sign-in
```

## Protected endpoint without token

```text
GET protected endpoint
(no Authorization)
→ 401
→ business handler not reached
```

## Protected endpoint with invalid token

```text
Authorization: Bearer invalid
→ 401
```

## Protected endpoint with valid USER token

```text
valid access token
USER_ROLE_USER
→ 403
```

## Protected endpoint with valid ADMIN token

```text
valid access token
USER_ROLE_ADMIN
→ endpoint succeeds
```

## Expired token

```text
expired access token
→ 401
→ Dashboard returns user to sign-in
```

## Sign out

```text
Dashboard
→ Sign Out
→ /sign-in
→ protected pages inaccessible
```

---

# 60. Do NOT Claim Browser Verification Unless It Actually Happened

If Cline cannot run a real browser against the complete local stack, say so.

Do not claim:

```text
"browser verified"
```

based solely on:

```text
go test
npm run build
curl
```

Those are valuable, but they are not the same as browser verification.

---

# 61. Final Report

The final Cline report must contain:

## A. Architecture

Explain the implemented authentication flow:

```text
Sign In
Refresh
Authorization
Sign Out
```

## B. Database

Show:

```text
users
access_tokens
```

and their important fields/relationships.

## C. Security

Explain:

* password hashing;
* salt;
* token generation;
* token storage;
* expiration;
* token validation;
* authorization middleware;
* role checking;
* secret logging protections.

Do NOT print real credentials or tokens.

## D. API

List:

```text
Sign In
Refresh
Sign Out
```

and all protected Dashboard endpoints.

## E. Dashboard

Explain:

* sign-in page;
* authentication context;
* route protection;
* API Authorization handling;
* sidebar user information;
* sign out.

## F. Public Exceptions

Explicitly list:

```text
InitiateDeposit
payments page / required payment endpoints
```

and explain how they remain available without admin authorization.

## G. Tests

Report exact results for:

```text
go build
go vet
go test
npm run build
npm run lint
protobuf generation
manual curl tests
browser tests, if actually performed
```

## H. Documentation

List every checkpoint/context document that was appended.

## I. Infrastructure

Explicitly state:

```text
ALB listener rules were NOT modified.
ALB YAML was NOT modified.
```

## J. Limitations

Clearly identify anything that could not be verified or anything deliberately deferred.

---

# Final Principle

This is a **minimal authentication implementation**, not a general authentication platform.

The desired architecture is:

```text
                    ┌──────────────────┐
                    │  Admin Dashboard  │
                    └────────┬─────────┘
                             │
                    email + password
                             │
                             ▼
                    ┌──────────────────┐
                    │ Clients Service  │
                    │                  │
                    │ Sign In          │
                    │ Refresh          │
                    │ Users            │
                    │ Access Tokens    │
                    └────────┬─────────┘
                             │
                    access token
                             │
             ┌───────────────┴───────────────┐
             ▼                               ▼
      ┌──────────────┐                ┌──────────────┐
      │   Clients    │                │ Transactions │
      │   Service    │                │   Service    │
      │              │                │              │
      │ Auth         │                │ Auth         │
      │ Middleware   │                │ Middleware   │
      └──────┬───────┘                └──────┬───────┘
             │                               │
             ▼                               ▼
       Admin endpoints                 Admin endpoints
```

The Dashboard is **not** the authority.

The Clients service/database is the authentication authority.

The Transactions service must enforce authentication before protected business logic.

The database must be authoritative for token validity.

And the infrastructure/ALB remains untouched.

**Stay inside the existing project's architecture. Read first. Implement minimally. Secure the credentials. Test everything. Append the documentation.**

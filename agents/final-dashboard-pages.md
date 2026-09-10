# Cline Agent — Implement Transactions, Disputes & Settings Dashboard Pages

## ROLE

You are a senior full-stack engineer working inside the RVPay repository.

You are running this task from the **repository root**.

Your job is to implement the remaining three Admin Dashboard pages:

1. Transactions
2. Disputes
3. Settings

The visual source of truth for these pages is provided as three PNG design files:

```text
admindashboard/new-pages/Transactions.png
admindashboard/new-pages/Disputes.png
admindashboard/new-pages/Settings.png
```

You must consume and inspect all three PNGs before implementing their corresponding pages.

You must also:

* wire the pages to the appropriate existing backend endpoints;
* identify and report backend endpoints that do not yet exist;
* create a detailed missing-endpoints report if backend work is required;
* ensure the pages integrate correctly with the existing Admin Dashboard;
* ensure Sidebar navigation works correctly;
* preserve the existing dashboard design system;
* preserve the existing environment-selection functionality on Settings;
* move the existing environment selector to the bottom of the new Settings page;
* fix the Success Modal redirect in `admindashboard/payment/page.tsx`;
* test the complete implementation;
* verify API routing/CORS behavior;
* verify compatibility with the existing AWS ALB routing;
* update all required project documentation/checkpoint/context files by APPENDING rather than overwriting;
* strictly obey the `.clineignore.md` and `.clinerules.md` files for every affected service.

Do not treat this as a greenfield frontend implementation.

The existing RVPay codebase, existing dashboard pages, existing backend APIs, existing visual language, existing routing, existing authentication, existing ALB routing, and the supplied PNG designs are the sources of truth.

---

# 1. ABSOLUTE PRIORITY ORDER

When making implementation decisions, use this order of authority:

1. Existing `.clinerules.md` for the affected service.
2. Existing `.clineignore.md` for the affected service.
3. Existing project context/checkpoint documentation.
4. Existing working dashboard pages/components and their established patterns.
5. Existing backend API contracts and protobuf definitions.
6. The supplied PNG designs for the new page-specific visual layout.
7. Existing application design tokens/components/styles.
8. Existing ALB routing configuration and path conventions.
9. Only then, reasonable implementation inference.

Do NOT invent a new design system.

Do NOT introduce arbitrary colors.

Do NOT introduce arbitrary typography.

Do NOT introduce arbitrary spacing systems.

Do NOT introduce arbitrary component styles.

Do NOT introduce new UI patterns when an existing dashboard pattern can be reused.

The PNG designs describe what these pages should look like.

The existing dashboard describes how that design should be implemented within the application.

---

# 2. REPOSITORY / SERVICE BOUNDARIES

You are operating from the repository root.

The three relevant services are:

```text
admindashboard/
clients/
transactions/
```

Treat them as separate services with their own rules and documentation.

Before making changes, locate and read the applicable:

```text
.clinerules.md
.clineignore.md
```

for:

```text
admindashboard/
clients/
transactions/
```

Also locate and consume the applicable:

```text
.project-context.md
.project-checkpoint.md
.service-checkpoint.md
README.md
```

and any other project/service-specific context files required by the existing rules.

Do not assume the files are named identically in all three services.

**Discover the actual files first.**

If a service uses additional Cline control/context files, consume those as required by that service's rules.

---

# 3. FIRST PHASE — REPOSITORY DISCOVERY

Before changing any code, perform a complete discovery pass.

Do not immediately start writing React/Next.js code.

Determine:

### Dashboard

* framework/version;
* App Router structure;
* current page routes;
* layout structure;
* Sidebar implementation;
* navigation implementation;
* shared components;
* shared styles;
* existing design tokens;
* existing icons;
* existing tables;
* existing cards;
* existing modals;
* existing filters;
* existing pagination;
* existing loading states;
* existing error states;
* existing empty states;
* API/request helpers;
* authentication/session handling;
* environment handling;
* current implementation of:

  * payment page;
  * payouts page;
  * sub-accounts page;
  * installation page;
  * any existing overview/dashboard page.

Pay particular attention to existing pages that are visually and functionally closest to the new designs.

### Transactions service

Inspect:

* HTTP routes;
* gRPC services;
* protobuf definitions;
* gateway routes;
* handlers;
* service methods;
* database models;
* repository methods;
* existing transaction endpoints;
* existing payment endpoints;
* existing payout endpoints;
* existing customer/merchant endpoints;
* existing overview/statistics endpoints;
* authentication middleware;
* CORS handling;
* request/response DTOs;
* tests.

### Clients service

Inspect:

* HTTP routes;
* gRPC services;
* protobuf definitions;
* gateway routes;
* handlers;
* authentication;
* user/session endpoints;
* environment-related functionality;
* any settings-related functionality;
* CORS handling;
* tests.

Do not modify anything during this discovery phase unless required to safely inspect/generated output.

---

# 4. READ THE PNG DESIGNS BEFORE IMPLEMENTATION

The three supplied files are:

```text
admindashboard/new-pages/Transactions.png
admindashboard/new-pages/Disputes.png
admindashboard/new-pages/Settings.png
```

These files are intentionally placed under:

```text
admindashboard/new-pages/
```

This directory must be added to the appropriate `.gitignore`.

Do NOT add the entire dashboard project to `.gitignore`.

Only add the supplied design-source directory/path:

```text
admindashboard/new-pages/
```

unless the existing `.gitignore` conventions require an equivalent precise pattern.

The PNGs are design references and should not be committed to the repository unless existing project rules explicitly require design assets to be committed.

---

# 5. VISUAL ANALYSIS REQUIREMENT

Inspect each PNG carefully before implementing it.

For each page determine:

* overall page structure;
* page width;
* content hierarchy;
* sidebar relationship;
* header;
* title/subtitle;
* cards;
* statistics;
* tables;
* table columns;
* badges;
* status indicators;
* filters;
* dropdowns;
* buttons;
* search;
* pagination;
* modals;
* tabs;
* empty states;
* spacing;
* alignment;
* borders;
* shadows;
* radius;
* typography hierarchy;
* iconography;
* responsive behavior implied by the design.

Do not merely use the PNG as inspiration.

Reproduce its structure as accurately as possible using the existing dashboard's components and styles.

If a visual element already exists elsewhere in the dashboard, reuse the existing implementation rather than creating a visually similar duplicate.

---

# 6. EXISTING DESIGN SYSTEM IS THE ONLY DESIGN SYSTEM

This is a critical requirement.

You must NOT introduce new:

* colors;
* fonts;
* gradients;
* shadows;
* border styles;
* button styles;
* badge styles;
* card styles;
* icon styles;
* spacing conventions;
* typography conventions.

If the PNG contains a color/style that appears to be part of the existing dashboard design language, identify the corresponding existing implementation/token and reuse it.

If a design element appears to require a color that does not exist anywhere in the dashboard, do NOT invent one.

Instead:

1. inspect the existing design system;
2. determine the closest existing dashboard token/style;
3. use the established dashboard styling;
4. document any unavoidable visual discrepancy in the checkpoint.

The goal is to make the new pages look like they belong to the same dashboard.

---

# 7. DO NOT REBUILD EXISTING DASHBOARD INFRASTRUCTURE

Before creating any component, determine whether an equivalent already exists.

Prefer:

```text
existing component
    ↓
existing shared component
    ↓
existing page pattern
    ↓
new component only if genuinely necessary
```

Do not create duplicate versions of:

* Sidebar;
* Header;
* authentication guards;
* API clients;
* request helpers;
* tables;
* pagination;
* modal infrastructure;
* buttons;
* cards;
* status badges;
* environment selector;
* loading indicators;
* error handling.

Extend existing infrastructure where appropriate.

---

# 8. IMPLEMENT TRANSACTIONS PAGE

Locate the current Transactions page.

The page already exists but is currently limited/placeholder-level.

Do not create a second Transactions page.

Replace/extend the existing implementation so that it matches:

```text
admindashboard/new-pages/Transactions.png
```

Determine the exact route from the existing dashboard routing structure.

Do not invent a new route if the route already exists.

Implement all visible functionality represented by the design.

For each piece of displayed data:

1. identify the appropriate backend source;
2. inspect existing endpoint contracts;
3. determine whether the endpoint already exists;
4. determine the correct request parameters;
5. determine authentication requirements;
6. determine expected response structure;
7. reuse the existing dashboard API/request mechanism.

Do not create fake/mock production data.

Do not hardcode values merely to make the screenshot look correct.

---

# 9. IMPLEMENT DISPUTES PAGE

Locate the current Disputes page/placeholder.

Do not create a duplicate route if one already exists.

Implement the page to match:

```text
admindashboard/new-pages/Disputes.png
```

Trace every required data element back to a real backend source.

Determine whether disputes are already represented in:

* Transactions service;
* protobuf definitions;
* HTTP gateway;
* database models;
* repositories;
* service methods;
* existing API routes.

If the required backend functionality exists, wire the dashboard to it.

If it does not exist, DO NOT invent an endpoint and pretend it exists.

Instead, document the missing functionality as described in Section 13.

---

# 10. IMPLEMENT SETTINGS PAGE

Locate the existing Settings page.

The Settings page already contains environment-selection functionality.

**Do NOT remove or replace that environment functionality.**

Instead:

1. implement the new Settings design from:

   ```text
   admindashboard/new-pages/Settings.png
   ```
2. preserve the existing environment-selection functionality;
3. move the environment selector to the **bottom of the new Settings page**;
4. preserve its existing behavior and persistence mechanism;
5. preserve the existing environment values and semantics;
6. do not create a second environment-management system.

The existing dashboard environment setup is authoritative.

Do not rename or redesign the environment mechanism unless the PNG explicitly requires a visual repositioning.

The task is to move it to the bottom of the Settings page while keeping the underlying behavior intact.

---

# 11. SIDEBAR / DASHBOARD FLOW

The new pages must integrate into the existing dashboard flow.

Inspect the existing Sidebar implementation before changing it.

Ensure:

* Transactions appears in the correct existing navigation location;
* Disputes appears in the correct existing navigation location;
* Settings appears in the correct existing navigation location;
* active navigation state works;
* clicking each item navigates to the correct page;
* switching from one page to another works without stale UI state;
* authentication guards continue to work;
* browser refresh on each route works;
* direct navigation to each route works;
* existing tabs/pages continue working;
* no existing Sidebar items are broken;
* no duplicate Sidebar entries are created;
* no hardcoded navigation mechanism is introduced if the project already has one.

Use the same navigation conventions as the existing dashboard.

Do not redesign the Sidebar.

---

# 12. API / BACKEND INTEGRATION

For every new page, create an explicit page-to-endpoint map during your investigation.

For example:

```text
Dashboard Page
    ↓
UI data requirement
    ↓
Backend service
    ↓
Endpoint / RPC
    ↓
Request
    ↓
Response
    ↓
Dashboard component
```

Do this separately for:

```text
Transactions
Disputes
Settings
```

### Existing endpoint first

Before creating any endpoint:

* search the Transactions service;
* search the Clients service;
* inspect protobuf;
* inspect gateway routes;
* inspect handlers;
* inspect service methods;
* inspect repository/database methods;
* inspect existing tests.

Do NOT create a duplicate endpoint when an existing endpoint already provides the required information.

Do NOT modify an existing endpoint's contract unnecessarily.

Prefer adapting the dashboard to an existing contract.

---

# 13. MISSING BACKEND ENDPOINTS

If a required endpoint does not exist, do not silently invent an implementation.

The agent must create:

```text
agents/new-pages-endpoints/
```

from the repository root if it does not already exist.

Create a detailed report there.

Use a descriptive filename such as:

```text
agents/new-pages-endpoints/README.md
```

or another appropriate filename if existing project conventions require it.

The report must identify every missing backend capability.

For each missing endpoint/functionality document:

### Endpoint requirement

* page:

  * Transactions
  * Disputes
  * Settings
* UI feature requiring it;
* owning service:

  * Clients
  * Transactions
* proposed HTTP route;
* proposed gRPC method if appropriate;
* HTTP method;
* authentication requirements;
* request parameters/body;
* response structure;
* pagination requirements;
* filtering requirements;
* sorting requirements;
* database information required;
* existing database tables/models that can support it;
* missing database model/migration if required;
* handler changes required;
* service-layer changes required;
* repository changes required;
* protobuf changes required;
* gateway changes required;
* CORS implications;
* ALB routing implications;
* testing requirements;
* recommended implementation sequence.

Clearly distinguish:

```text
EXISTS
```

from:

```text
MISSING
```

from:

```text
PARTIALLY EXISTS
```

Do not implement speculative backend functionality merely because the UI design contains it.

The report must be sufficiently detailed that a future Cline agent can implement the missing backend work without repeating the entire investigation.

---

# 14. CORS — CRITICAL

CORS behavior is extremely important for this task.

The dashboard already communicates with the backend through the existing API architecture.

Do NOT bypass that architecture.

Do NOT:

* disable CORS;
* add wildcard origins;
* remove authentication headers;
* create browser-side proxy hacks;
* introduce Next.js rewrites merely to hide CORS problems;
* hardcode tokens;
* bypass the existing API request helper.

Instead, use the existing dashboard API mechanism.

Verify that every new request:

1. reaches the correct public API host;
2. uses the correct `/v1/public/...` route conventions;
3. includes authentication where required;
4. handles OPTIONS preflight where applicable;
5. receives the correct CORS headers;
6. does not produce a browser CORS error;
7. does not accidentally call an internal/private service address from the browser.

Use the existing CORS middleware/configuration.

If a new endpoint requires a CORS change, document it and implement it only if the endpoint itself is being implemented in this task.

---

# 15. AWS ALB ROUTING — CRITICAL

The public API is routed through the AWS Application Load Balancer.

Existing testing routing includes:

```text
25
api.rvpay.xyz
/v1/public/clients*
/v1/public/integrations*
/v1/public/platforms*
→ Clients
```

```text
50
api.rvpay.xyz
/v1/public/merchants*
/v1/public/deposits*
/v1/public/customers*
→ Transactions
```

```text
75
api.rvpay.xyz
/v1/public/transactions*
/v1/public/payouts*
/v1/public/payments*
→ Transactions
```

```text
125
api.rvpay.xyz
/payments/custom-provider*
/oauth/callback*
→ Clients
```

```text
150
api.rvpay.xyz
/v1/public/auth*
→ Clients
```

The `150` auth rule was manually added to the testing environment because it was missing from the ALB configuration.

**Do not modify the ALB/listener YAML or the `aws-cloudformation-templates` submodule unless explicitly authorized.**

The agent must understand the existing ALB routing model before adding any backend route.

For every endpoint required by these pages, determine:

* which service owns it;
* which public path it uses;
* whether the current ALB path rules already route that path;
* whether the path can safely fit an existing rule;
* whether a new ALB rule would be required.

If an endpoint requires a new ALB listener rule and that rule cannot be implemented without modifying protected infrastructure files, document it in:

```text
agents/new-pages-endpoints/
```

with:

```text
ALB ROUTING CHANGE REQUIRED
```

Do not silently create a route that will be unreachable through the public API.

Do not modify existing ALB listener YAML.

---

# 16. IMPORTANT ALB PATH CONVENTION

Existing public API routing uses service-oriented path prefixes.

Clients currently uses paths such as:

```text
/v1/public/clients*
/v1/public/integrations*
/v1/public/platforms*
/v1/public/auth*
```

Transactions currently uses paths such as:

```text
/v1/public/merchants*
/v1/public/deposits*
/v1/public/customers*
/v1/public/transactions*
/v1/public/payouts*
/v1/public/payments*
```

If a missing endpoint is proposed, follow the existing API naming conventions.

Do not create arbitrary paths such as:

```text
/api/disputes
/dashboard/transactions
/settings-data
```

unless the existing backend architecture explicitly uses that convention.

---

# 17. AUTHENTICATION

The new dashboard pages must use the existing dashboard authentication/session infrastructure.

Do not implement another login/token mechanism.

Use the existing:

* AuthProvider;
* AuthGuard;
* bearer access token handling;
* refresh handling;
* API request helper;
* 401 retry/single-flight refresh behavior.

Protected endpoints must remain protected.

Do not make a dashboard-only endpoint public merely because the UI needs it.

---

# 18. SETTINGS — ENVIRONMENT HANDLING

The environment selector already exists.

Find exactly how it currently works.

Determine:

* where the environment is stored;
* the existing storage key;
* how requests use it;
* how the UI reads it;
* what values are supported;
* whether switching environments reloads data;
* whether the existing implementation has any side effects.

Preserve this behavior.

The known dashboard environment storage key is:

```text
rvpay-dashboard-environment
```

Do not replace it with another key.

The selector should simply be visually relocated to the bottom of the new Settings page.

---

# 19. PAYMENT SUCCESS MODAL FIX

As a minor independent task, inspect:

```text
admindashboard/payment/page.tsx
```

Find the Success Modal redirect URL.

Change the redirect destination to:

```text
https://store.citscm.com
```

Do not otherwise redesign or refactor the payment page.

Do not change unrelated payment behavior.

After making the change, test that the Success Modal still behaves exactly as before except for the destination URL.

---

# 20. RESPONSIVENESS

Implement the new pages using the existing dashboard responsive conventions.

Do not invent a new responsive framework.

Inspect existing pages to determine how they behave at:

* desktop;
* tablet;
* narrow viewport.

The implementation must not break the existing dashboard shell or Sidebar.

If the PNGs only depict desktop designs, preserve the desktop visual match while ensuring the page does not introduce obvious overflow/broken layout at smaller widths.

---

# 21. ERROR / LOADING / EMPTY STATES

Use existing dashboard conventions.

For every endpoint-backed component, account for:

* loading;
* successful data;
* empty result;
* API error;
* unauthorized/expired session;
* retry where existing patterns support retry.

Do not replace real loading/error states with fake data.

Do not introduce a new error-handling architecture.

---

# 22. DATA FORMAT / DISPLAY

Do not alter backend data merely for visual convenience.

If the API returns:

```text
timestamps
amounts
statuses
IDs
currency
customer data
```

format them at the dashboard presentation layer using existing dashboard conventions.

Reuse existing:

* currency formatting;
* date formatting;
* status mapping;
* pagination;
* number formatting.

If an existing helper exists, use it.

---

# 23. NO MOCK DATA

This is a production dashboard.

Do not use:

```text
mockData
dummyData
fakeTransactions
fakeDisputes
placeholderStats
hardcoded API responses
```

except in tests where the project's existing testing conventions require fixtures/mocks.

The actual page implementation must use the real backend.

---

# 24. GENERATED FILES

Follow the existing project rules concerning generated files.

If protobuf/gateway/generated files are generated through a repository-defined process:

* do not hand-edit generated files;
* identify the source file;
* run the project's generation command;
* verify the generated result.

If no backend implementation is required, do not regenerate unrelated code.

---

# 25. TESTING REQUIREMENTS

Testing is mandatory.

At minimum:

### Dashboard

Run the appropriate:

```text
install/dependency check
lint
typecheck
build
test
```

commands according to the existing dashboard package configuration and Cline rules.

### Transactions

Run the appropriate:

```text
go test
go vet
build
```

and any service-specific validation required by `.clinerules.md`.

### Clients

Run the appropriate:

```text
go test
go vet
build
```

and any service-specific validation required by `.clinerules.md`.

Do not skip tests because the changes are "mostly frontend."

---

# 26. END-TO-END API VERIFICATION

Where the environment permits, verify that the dashboard actually calls the intended API routes.

For each new page:

```text
open page
    ↓
observe API request
    ↓
confirm host
    ↓
confirm path
    ↓
confirm HTTP method
    ↓
confirm authentication
    ↓
confirm response
    ↓
confirm UI rendering
```

Verify that requests are going through:

```text
https://api.rvpay.xyz
```

or the existing environment-specific API mechanism.

Do not accidentally call:

```text
localhost
private ECS address
internal service DNS
container address
```

from the browser.

---

# 27. CORS END-TO-END VERIFICATION

Specifically verify requests that trigger browser preflight.

For relevant requests, confirm:

```text
OPTIONS
    ↓
ALB
    ↓
correct target group
    ↓
Go service
    ↓
CORS middleware
    ↓
Access-Control-Allow-Origin
```

The dashboard origin is:

```text
https://admindashboard.rvpay.xyz
```

The API must not return the ALB fallback:

```text
503 Unknown Host
```

If a CORS error appears, diagnose the routing path rather than immediately changing CORS code.

Remember the previous RVPay issue:

```text
503 Unknown Host
server: awselb/2.0
```

was caused by the missing ALB `/v1/public/auth*` rule, not by the Go CORS middleware.

---

# 28. ALB MONITORING / ROUTING VERIFICATION

For every new backend endpoint that is actually implemented, verify its relationship to the existing ALB rules.

The agent must explicitly record:

```text
Endpoint
Owner service
Public route
Existing ALB rule that catches it
OR
New ALB rule required
```

If a new rule is required, document it.

Do not modify the protected ALB CloudFormation YAML.

---

# 29. SIDEBAR FLOW TEST

Manually or through available browser tooling/test procedures, verify:

```text
Dashboard
  ↓
Transactions
  ↓
Disputes
  ↓
Settings
  ↓
Transactions
  ↓
Existing page
  ↓
Settings
```

Verify:

* correct URL;
* correct active state;
* correct page content;
* no full application crash;
* no stale page state;
* no authentication loss;
* no broken Sidebar;
* no console errors attributable to the implementation.

Also directly load each route after refreshing the browser.

---

# 30. VISUAL QA

Compare each implementation against its PNG:

```text
Transactions.png
Disputes.png
Settings.png
```

Perform a visual QA pass.

Check:

* dimensions;
* spacing;
* hierarchy;
* alignment;
* cards;
* tables;
* badges;
* buttons;
* icons;
* typography;
* colors;
* borders;
* shadows;
* page density;
* Sidebar integration;
* responsive behavior.

Do not settle for "functionally similar."

The implementation should visually match the provided design as closely as possible while using the existing dashboard's components and design system.

If the screenshot and existing dashboard design system conflict, preserve the existing design system and document the specific discrepancy.

---

# 31. DOCUMENTATION / CHECKPOINT UPDATES

This task must include all required project documentation updates.

Do not overwrite existing checkpoint files.

**Append to them.**

Follow the exact conventions specified by each service's:

```text
.clinerules.md
.clineignore.md
```

and existing checkpoint/context documentation.

At minimum, identify and update the appropriate files for:

```text
admindashboard
transactions
clients
```

where those services were actually inspected or modified.

These files will usually be:
```text
.clinecheck.md
.project-checkpoint.md
.service-checkpoint.md
```

Record:

* task performed;
* pages implemented;
* files changed;
* components created/modified;
* routes added/modified;
* backend endpoints consumed;
* missing endpoints discovered;
* API contracts used;
* authentication considerations;
* CORS verification;
* ALB routing verification;
* testing performed;
* test results;
* build results;
* visual QA;
* known limitations;
* follow-up work;
* environment settings behavior;
* payment Success Modal redirect fix.

Do not erase previous agent history.

Do not replace the checkpoint with a new summary.

Append a new dated/task-specific section according to the existing checkpoint convention.

---

# 32. PROJECT CONTEXT UPDATES

If the service's project-context file is designed to track architecture/current state, update it according to its existing conventions.

Do not turn project-context files into giant task logs if their existing purpose is architectural context.

Keep additions concise and architectural.

For example, record:

* new dashboard routes;
* endpoint ownership;
* newly consumed backend APIs;
* missing backend functionality;
* relevant ALB routing dependencies.

---

# 33. README UPDATES

Inspect each affected service README before changing it.

Only update README content where the existing README conventions indicate that the change belongs there.

Do not add unnecessary documentation.

If the existing project workflow requires checkpoint/README updates after every agent, comply with that existing rule.

---

# 34. .GITIGNORE REQUIREMENT

Add:

```text
admindashboard/new-pages/
```

to the appropriate root/dashboard `.gitignore` according to the repository's existing conventions.

The three PNG files are design input and should not accidentally become tracked source files.

Verify with:

```bash
git status
```

that the PNG directory is ignored.

Do not add unrelated files to `.gitignore`.

---

# 35. FILE CONSUMPTION MATRIX

Before implementation, create an internal matrix similar to:

| Service        | Rules | Context | Checkpoint | README | Relevant Source                          |
| -------------- | ----- | ------- | ---------- | ------ | ---------------------------------------- |
| admindashboard | READ  | READ    | READ       | READ   | pages/components/API/sidebar             |
| transactions   | READ  | READ    | READ       | READ   | routes/proto/handlers/service/repository |
| clients        | READ  | READ    | READ       | READ   | routes/proto/handlers/service/repository |

Then identify exactly which files were consumed.

At the end of the task, ensure every required file from the service's own rules has been consumed.

Do not assume a file can be skipped because the task "looks frontend-only."

---

# 36. CHANGE DISCIPLINE

Make the smallest coherent changes necessary.

Do not:

* refactor unrelated code;
* upgrade dependencies;
* change framework versions;
* rename unrelated files;
* alter database schema unless genuinely required;
* change authentication architecture;
* change ALB infrastructure;
* change environment semantics;
* redesign existing pages;
* redesign the Sidebar;
* change the API architecture;
* introduce a new state-management library;
* introduce a new UI library;
* introduce a new CSS framework.

If you encounter something unrelated but problematic, document it rather than fixing it unless it blocks this task.

---

# 37. BACKEND IMPLEMENTATION BOUNDARY

The primary task is to implement the dashboard pages and wire them to existing backend endpoints.

If a backend endpoint is missing:

**Do not automatically expand this task into a large backend feature implementation.**

Instead:

1. determine exactly what is missing;
2. determine the correct owning service;
3. document the implementation requirements;
4. document the ALB/CORS implications;
5. place the report under:

   ```text
   agents/new-pages-endpoints/
   ```

Only implement backend changes if:

* the required endpoint is clearly part of an existing capability;
* the existing architecture makes the implementation unambiguous;
* the service's Cline rules permit it;
* doing so is necessary for the page to function;
* the change can be completed safely within this task.

Otherwise report it for the next backend agent.

---

# 38. SUCCESS CRITERIA

This task is complete only when:

### Transactions

* [ ] Existing Transactions route identified.
* [ ] Placeholder implementation replaced with the PNG-matching implementation.
* [ ] Real backend endpoints wired.
* [ ] Loading/error/empty states implemented using existing patterns.
* [ ] Authentication works.
* [ ] CORS works.
* [ ] ALB routing verified.
* [ ] Visual QA completed.

### Disputes

* [ ] Existing Disputes route identified.
* [ ] PNG-matching implementation completed.
* [ ] Real backend endpoints wired where available.
* [ ] Missing backend capabilities documented.
* [ ] CORS implications documented/verified.
* [ ] ALB implications documented/verified.
* [ ] Visual QA completed.

### Settings

* [ ] Existing Settings route identified.
* [ ] PNG-matching implementation completed.
* [ ] Existing environment functionality preserved.
* [ ] Existing environment storage mechanism preserved.
* [ ] Environment selector moved to bottom of page.
* [ ] Visual QA completed.

### Sidebar

* [ ] Transactions navigation works.
* [ ] Disputes navigation works.
* [ ] Settings navigation works.
* [ ] Active states work.
* [ ] Existing navigation remains intact.
* [ ] Direct route loading works.
* [ ] Refresh works.

### Payment

* [ ] `admindashboard/payment/page.tsx` Success Modal redirects to:
  `https://store.citscm.com`
* [ ] No unrelated payment behavior changed.

### Backend

* [ ] Existing endpoints reused wherever possible.
* [ ] No duplicate endpoints created.
* [ ] Missing endpoints documented.
* [ ] `agents/new-pages-endpoints/` created/updated where required.

### Infrastructure

* [ ] Existing ALB rules inspected.
* [ ] Endpoint routing ownership documented.
* [ ] No protected ALB YAML modified.
* [ ] No `aws-cloudformation-templates` submodule modified.
* [ ] CORS verified.
* [ ] ALB routing verified.

### Documentation

* [ ] Required `.cline`/project context files consumed.
* [ ] Required checkpoint files updated.
* [ ] Checkpoints appended, never overwritten.
* [ ] Required README updates made.
* [ ] `.gitignore` updated for `admindashboard/new-pages/`.
* [ ] `git status` checked.

### Testing

* [ ] Dashboard lint/typecheck/tests/build pass according to project rules.
* [ ] Transactions tests/build/vet pass according to project rules.
* [ ] Clients tests/build/vet pass according to project rules.
* [ ] New pages manually/end-to-end verified where tooling permits.
* [ ] No new browser console errors attributable to these changes.

---

# 39. FINAL REPORT

At the end, provide a concise but complete report containing:

## Implemented

List:

* Transactions
* Disputes
* Settings
* Sidebar integration
* Payment Success Modal redirect

## Backend endpoints consumed

For each endpoint:

```text
Method
Path/RPC
Owning service
Purpose
Authentication
```

## Missing backend functionality

For each missing item:

```text
Page
Feature
Owning service
Why missing
Recommended endpoint/RPC
Required backend work
ALB requirement
CORS requirement
```

Point to the generated:

```text
agents/new-pages-endpoints/
```

report.

## Files changed

Group by:

```text
admindashboard/
transactions/
clients/
root/infrastructure/docs
```

## Tests

Report the exact commands run and their results.

## API/CORS verification

State which endpoints were tested and whether they successfully passed through the public API path without browser CORS failures.

## ALB verification

For every endpoint used, state:

```text
endpoint → service → ALB rule/path
```

and identify any route that still requires an infrastructure change.

## Documentation

List every checkpoint/context/README file updated.

Explicitly confirm:

```text
checkpoint files were appended, not overwritten.
```

## Known limitations

Only list real limitations discovered during implementation.

Do not hide missing backend functionality.

---

# 40. FINAL NON-NEGOTIABLE RULES

1. **PNG designs are the visual source of truth for the three new pages.**
2. **Existing dashboard pages are the implementation-pattern source of truth.**
3. **Do not invent colors or design elements.**
4. **Do not create duplicate shared components.**
5. **Do not invent backend endpoints when existing endpoints can be reused.**
6. **Do not use mock production data.**
7. **Do not bypass the existing authentication/API architecture.**
8. **CORS must be tested.**
9. **ALB routing must be checked for every new API route.**
10. **Do not modify protected ALB CloudFormation YAML.**
11. **Do not modify the `aws-cloudformation-templates` submodule.**
12. **Preserve the existing Settings environment functionality.**
13. **Move the environment selector to the bottom of Settings.**
14. **Change the payment Success Modal redirect only to `https://store.citscm.com`.**
15. **Add `admindashboard/new-pages/` to `.gitignore`.**
16. **Consume the rules/context/checkpoint/README files for Dashboard, Transactions and Clients.**
17. **Respect every `.clineignore.md` and `.clinerules.md`.**
18. **Append to checkpoint files; never overwrite historical agent work.**
19. **Document missing backend functionality under `agents/new-pages-endpoints/`.**
20. **Do not declare success until tests, API routing, CORS, ALB compatibility, Sidebar flow, and visual QA have been checked.**

Begin with repository discovery and rule/context consumption. Do not start implementation until that discovery is complete.

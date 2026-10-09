Cline, make a targeted correction to `admindashboard/app/.../page.tsx` (the current payments page / `page.tsx` file).

## IMPORTANT — DOCUMENT HAS CHANGED

The `page.tsx` document has changed since you last worked on it.

**Do NOT rely on your previous version, previous assumptions, or previous implementation.**

First, re-read the CURRENT files from the repository, especially:

* `admindashboard/.clinerules.md`
* `admindashboard/.clineignore.md`
* `admindashboard/.project-checkpoint.md`
* `admindashboard/.project-context.md`
* `admindashboard/README.md`
* the CURRENT payments `page.tsx`
* any directly relevant payment/client/API code in the admindashboard
* the backend/API contract already used by RVPay

You MUST adhere strictly to `.clinerules.md` and `.clineignore.md`.

Do not modify files outside the scope required for this task.

---

# OBJECTIVE

There are exactly TWO things to fix.

### 1. SUSPEND `payment_initiate_props`

The HighLevel `payment_initiate_props` iframe handshake is currently not delivering the payment context.

The current browser console proves:

```text
[RVPay] Payment iframe initialized
[RVPay] window === window.parent: false
[RVPay] current origin: https://admindashboard.rvpay.xyz
[RVPay] parent origin: https://api.rvamply.com/
[RVPay] Sending ready event: { type: "custom_provider_ready", loaded: true }
[RVPay] Ready event sent
```

There is NO inbound `payment_initiate_props` message.

Therefore:

**Do NOT spend time trying to repair or depend on `payment_initiate_props`.**

Temporarily suspend that mechanism.

You may leave the logging/listener code in place if it is useful for future reactivation, but it MUST NOT be required for the payment flow to proceed.

Do not delete useful diagnostics unless they are genuinely unnecessary.

---

# 2. MAKE `transactionId` THE AUTHORITATIVE PAYMENT IDENTIFIER

The payment page must obtain and use the `transactionId` that HighLevel has already supplied as part of the payment flow.

The current payment flow is blocked because the page waits for:

```text
payment_initiate_props.transactionId
```

That must stop.

Instead, inspect the CURRENT application/payment integration and determine exactly where/how the `transactionId` is already available to the payment page.

Use the existing transaction/payment URL/query/context mechanism if that is already present.

Do NOT invent a new transaction ID.

Do NOT generate one on the frontend.

Do NOT use a random UUID.

Do NOT use `orderId` as a replacement.

Do NOT fabricate payment context.

The page should extract the real HighLevel/RVPay transaction ID from the existing payment flow.

For example, if the current HighLevel payment URL contains something such as:

```text
?transactionId=...
```

then read it safely from the browser URL using the appropriate Next.js client-side mechanism.

Inspect the existing project before deciding the exact implementation.

---

# PAYMENT FLOW REQUIREMENT

Once the real `transactionId` is available, the page MUST actually continue into the RVPay backend/microservices payment flow.

The current implementation is NOT sufficient.

Currently it effectively does:

```text
queryPaymentStatus()
    ↓
if verified
    ↓
start polling queryPaymentStatus()
```

That does NOT initiate a payment.

The page currently contains a comment explicitly acknowledging that payment initiation is missing.

That must be fixed.

The intended flow should be:

```text
HighLevel
   │
   │ transactionId
   ▼
RVPay Payment Page
   │
   │ payment request
   ▼
RVPay public/backend API
   │
   ▼
RVPay microservices
   │
   ├── transaction/payment validation
   ├── provider/payment initiation
   └── persistence/state update
   │
   ▼
mobile-money provider
   │
   ▼
callback/status update
   │
   ▼
RVPay microservices
   │
   ▼
Payment Page status polling
```

The frontend must therefore call the EXISTING backend endpoint/contract that actually initiates the payment.

---

# CRITICAL: DO NOT INVENT AN API

Before changing `page.tsx`:

1. Search the repository for the existing payment/deposit initiation API.
2. Search for:

   * transaction creation
   * deposit creation
   * payment initiation
   * custom provider
   * transactionId
   * query
   * initiate
   * deposit
   * pawaPay
   * payments/custom-provider
3. Inspect the backend/API contract already used by RVPay.
4. Identify the existing public endpoint intended to initiate this payment.
5. Use that existing endpoint from `page.tsx`.

Do NOT invent a new endpoint such as:

```text
/payments/custom-provider/initiate
```

unless that endpoint ACTUALLY EXISTS in the repository/backend.

Do not modify backend microservices as part of this task unless `.clinerules.md` explicitly requires it and the existing backend contract proves that a minimal compatibility change is unavoidable.

This task is primarily a `page.tsx` correction.

---

# PAYMENT REQUEST PAYLOAD

The initiation request must contain the real transaction context required by the existing backend contract.

At minimum, preserve/use:

```text
transactionId
amount
currency
country/provider
phone
```

and any other fields the existing backend contract explicitly requires.

Do not send fake/null placeholder values merely to make TypeScript compile.

Do not expose server-side secrets/API keys in the browser.

The existing:

```text
publishableKey
apiKey
```

handling must NOT accidentally expose a server-side credential.

Follow the existing backend contract and `.clinerules.md`.

---

# PAYMENT STATUS

After successful initiation:

1. Do NOT immediately declare payment successful.
2. Start/continue status polling using the existing query/status endpoint.
3. Use the same `transactionId`.
4. Poll until:

   * successful
   * failed
   * or an appropriate timeout/error condition occurs.
5. Always clean up the polling interval.
6. Do not create multiple polling intervals.
7. Do not allow duplicate payment submissions while processing.

The UI should clearly communicate states such as:

```text
Verifying payment request...
Initiating payment...
Waiting for mobile money confirmation...
Payment completed successfully.
Payment failed.
Unable to process payment.
```

Use the existing backend's actual response semantics rather than inventing new ones.

---

# IMPORTANT: PHONE + PROVIDER DATA

The page already has:

* country selection
* provider selection
* phone number
* amount
* currency

Preserve the existing UI and behavior.

Do NOT redesign the page.

Do NOT remove the provider selection.

Do NOT remove the country selection.

Do NOT remove the terms checkbox.

Do NOT remove useful console diagnostics.

Only change what is necessary to repair the payment flow.

When constructing the backend initiation request, map these existing UI values to the backend's actual expected fields.

Pay particular attention to the mobile-money phone number:

```text
country dial code + phone number
```

If the backend expects an international number, construct it according to the existing backend contract rather than sending an ambiguous local number.

---

# TRANSACTION ID SOURCE

Implement a robust transaction ID lookup based on the CURRENT application's actual payment URL/context.

The preferred order should be:

1. Existing transaction ID URL/query parameter supplied by HighLevel/RVPay.
2. Existing application payment context mechanism, if already implemented.
3. `payment_initiate_props` ONLY as a diagnostic/future fallback — NOT as a requirement for today's payment flow.

The page MUST be capable of reaching the backend without receiving:

```text
payment_initiate_props
```

If no transaction ID exists at all, fail safely with:

```text
Invalid payment request. Missing payment transaction information.
```

Do NOT initiate a payment without a real transaction ID.

---

# SUSPEND, DON'T DESTROY, THE HIGHLEVEL HANDSHAKE

Keep the existing diagnostics around the iframe if they are useful.

For example, retain useful logs such as:

```text
[RVPay] Payment iframe initialized
[RVPay] window === window.parent: false
[RVPay] current origin: ...
[RVPay] parent origin: ...
[RVPay] Sending ready event: ...
[RVPay] Ready event sent
```

But make it explicit in the code that:

```text
payment_initiate_props
```

is currently suspended/non-authoritative.

Do NOT let missing `payment_initiate_props` prevent payment.

If the existing `custom_provider_ready` message is required by HighLevel, KEEP IT.

The problem is not the ready event itself.

The problem is that the page currently depends on HighLevel returning `payment_initiate_props`.

---

# SECURITY

Do NOT:

* expose client secrets
* expose provider secrets
* put server credentials in `NEXT_PUBLIC_*`
* trust arbitrary origins unnecessarily
* accept arbitrary transaction IDs from untrusted sources without the backend validating them
* bypass backend transaction validation
* mark payments successful from frontend state alone

The backend/microservices must remain authoritative for payment state.

The frontend only initiates the existing backend transaction and displays its state.

---

# NEXT.JS / TYPESCRIPT REQUIREMENTS

The page is a client component.

Use the correct Next.js client-side URL/query mechanism for the version/configuration already used by this project.

Avoid:

```text
window.location
```

during server rendering.

Do not introduce SSR/build-time failures.

Do not introduce dependencies unless absolutely necessary.

Keep the implementation compatible with the project's existing Next.js version.

Avoid hydration mismatches.

---

# BUILD AND PUBLISH REQUIREMENT

This is extremely important.

The resulting `page.tsx` MUST NOT hold up the deployment Build and Publish phase.

Before finishing, run the project's existing validation/build commands specified by `.clinerules.md`.

At minimum verify:

```text
TypeScript compilation
Next.js production build
lint, if configured
```

Use the repository's existing commands rather than inventing a new build process.

Check specifically for:

* TypeScript errors
* invalid imports
* server/client boundary errors
* `window` usage during build
* `useSearchParams` / Suspense requirements if applicable
* missing environment variables
* unreachable code
* implicit `any`
* malformed JSX
* API payload type errors

If the project uses a Docker production build, verify that this page does not cause the Docker Build/Publish stage to fail.

DO NOT add a workaround that merely suppresses build errors.

The final code must genuinely compile.

---

# SCOPE CONTROL

Only target these two problems:

## Problem A

HighLevel `payment_initiate_props` is not arriving.

Solution:

```text
Suspend dependency on payment_initiate_props.
Use the real transactionId already available through the existing payment flow.
```

## Problem B

The page currently verifies/polls but does not actually initiate the payment through the backend.

Solution:

```text
Call the EXISTING RVPay payment/deposit initiation endpoint
with the real transactionId and required payment information,
then poll the existing status endpoint.
```

Do NOT:

* redesign the UI
* rewrite unrelated components
* change the backend architecture
* create new microservices
* create new endpoints without evidence
* modify the PawaPay SDK
* modify unrelated RVPay services
* change authentication architecture
* change Docker architecture
* add unnecessary dependencies

---

# ACCEPTANCE CRITERIA

Do not report completion until ALL of these are true:

### A. Build

The current `page.tsx` passes the project's production build.

### B. Transaction ID

The page can obtain the actual transaction ID without:

```text
payment_initiate_props
```

arriving.

### C. No fake transaction IDs

No generated/random transaction IDs.

### D. Initiation

Clicking:

```text
Pay <amount><currency>
```

actually sends a request to the existing RVPay backend payment/deposit initiation API.

### E. Backend

The backend receives the real:

```text
transactionId
```

plus the required payment information according to its existing contract.

### F. Status

After initiation, the page polls the existing status endpoint using that same transaction ID.

### G. Success/failure

The page correctly handles backend success and failure.

### H. Duplicate prevention

Repeated clicks cannot initiate duplicate payments while one is processing.

### I. Diagnostics

Keep the useful `[RVPay]` iframe/payment diagnostics.

### J. HighLevel

Missing:

```text
payment_initiate_props
```

does NOT block the payment.

### K. Deployment

No new build/publish blocker is introduced.

---

# REQUIRED FINAL REPORT

When finished, report:

1. Exact file changed.
2. How the transactionId is now obtained.
3. Which EXISTING backend endpoint is used to initiate the payment.
4. Exact initiation payload shape at a high level.
5. Which existing endpoint is used for status polling.
6. What happened to `payment_initiate_props`.
7. Build/lint/typecheck results.
8. Any remaining limitation.

Do not claim the payment flow is fixed unless you have verified the actual code path from:

```text
Pay button
→ backend initiation request
→ transactionId
→ status polling
```

Also explicitly distinguish between:

```text
verified locally by build/code inspection
```

and

```text
verified against a live HighLevel payment
```

because a successful production build is not proof that HighLevel actually sends the expected request.

Proceed now. Do not ask me to restate the current `page.tsx`; it has already been provided, but remember that the repository document has changed and you MUST inspect the current file before editing it.

Cline, make a targeted correction to the CURRENT `admindashboard/app/payment/page.tsx` based on a direct comparison with the known-working HighLevel custom payment-provider implementation in the repository/project.

**IMPORTANT — THE CURRENT FILE IS AUTHORITATIVE**

The current `admindashboard/app/payment/page.tsx` has changed.

Do NOT rely on an older version of the file.

Before editing, re-read:

* `admindashboard/.clinerules.md`
* `admindashboard/.clineignore.md`
* `admindashboard/.project-checkpoint.md`
* `admindashboard/.project-context.md`
* `admindashboard/README.md`
* CURRENT `admindashboard/app/payment/page.tsx`
* all directly relevant payment/client/API code in `admindashboard`

Also locate and inspect the **known-working HighLevel custom payment implementation** discussed in the project, particularly the `/payment/checkout` implementation.

The purpose of this task is to compare the two implementations and reproduce the proven HighLevel iframe/payment-context behavior in the current `/payment` page.

Do not modify unrelated code.

---

# OBJECTIVE

Fix the actual integration discrepancy between:

1. the current `admindashboard/app/payment/page.tsx`

and

2. the known-working `/payment/checkout` HighLevel custom-provider implementation.

The issue is NOT Pawapay.

The issue is:

```text
HighLevel
   ↓
iframe custom-provider handshake
   ↓
payment context
   ↓
transactionId
   ↓
payment initiation request
   ↓
RVPay backend
```

The current page is not reproducing the working HighLevel integration correctly.

---

# CRITICAL FINDING FROM THE COMPARISON

The known-working implementation does NOT merely send one `custom_provider_ready` message and wait.

Its behavior is materially different.

The working implementation:

1. Sends `custom_provider_ready` repeatedly.
2. Uses approximately a 500ms retry interval.
3. Attempts the ready message multiple times.
4. Sends the ready payload as a JSON-stringified message.
5. Includes:

```text
type: "custom_provider_ready"
loaded: true
addCardOnFileSupported: false
```

6. Its `message` listener handles both string and object `event.data`.
7. It parses JSON-stringified message payloads.
8. It accepts:

```text
payment_initiate_props
```

and:

```text
setup_initiate_props
```

9. It extracts the actual HighLevel payment context, including:

```text
transactionId
orderId
amount
currency
contact
locationId
```

10. That payment context is then used by the payment flow.

The current implementation instead does essentially:

```ts
window.parent.postMessage(
  {
    type: "custom_provider_ready",
    loaded: true,
  },
  "*"
);
```

only once.

Its listener also assumes that `event.data` is already an object.

This is a critical difference.

---

# DO NOT "SUSPEND" THE HANDSHAKE

Correct the previous assumption.

Do NOT remove the HighLevel handshake.

Do NOT make `payment_initiate_props` permanently non-authoritative.

Do NOT replace the GHL transaction context with a fabricated value.

The known-working repository demonstrates that the GHL handshake is capable of providing the payment context.

The task is to make the current page behave like the working implementation.

The current console observation that one ready message did not result in an inbound event is NOT sufficient evidence that the handshake itself is unsupported.

The working implementation's retry/stringification/parsing behavior must be reproduced first.

---

# STEP 1 — INSPECT THE WORKING IMPLEMENTATION

Before modifying `page.tsx`, locate the known-working implementation.

Search the repository for:

```text
custom_provider_ready
payment_initiate_props
setup_initiate_props
addCardOnFileSupported
postMessage
payment/checkout
transactionId
```

Identify the exact working implementation.

Do NOT rely solely on this instruction's description.

Read the actual source.

Determine:

* where the ready message is sent
* how often it is sent
* how many attempts are made
* whether it is stringified
* how incoming messages are parsed
* which event types are accepted
* how `transactionId` is stored
* how amount/currency/contact/location are propagated
* how the payment form consumes the context
* whether there is a server route involved
* whether the working page sends the transaction context to an existing backend route

The working implementation is the reference behavior.

---

# STEP 2 — REPAIR THE GHL IFRAME HANDSHAKE

Modify the current `/payment` page so its handshake follows the working implementation.

The page should:

1. Register its `message` listener before sending the ready message.

2. Send the ready message using the same representation as the working implementation.

If the working implementation uses:

```ts
JSON.stringify(...)
```

then reproduce that behavior.

3. Include the same supported fields as the working implementation, including:

```text
type
loaded
addCardOnFileSupported
```

when appropriate.

4. Retry the ready message using the same retry behavior as the working implementation.

The purpose is to handle timing/race conditions where HighLevel has not yet installed its message listener when the iframe initially announces readiness.

Do not invent a new retry architecture if the working implementation already contains one.

Reuse the established pattern.

5. Stop retrying once the appropriate success/context condition occurs, or use the exact lifecycle behavior already present in the working implementation.

6. Clean up the retry timer on component unmount.

Do not leak intervals/timers.

---

# STEP 3 — MAKE MESSAGE PARSING MATCH THE WORKING IMPLEMENTATION

The current code currently does:

```ts
const data = event.data;

if (!data || typeof data === "string") {
  return;
}
```

That is potentially wrong for the actual GHL integration.

If the working implementation receives JSON-stringified messages, the current implementation simply throws those messages away.

Change this to follow the working implementation.

The handler must safely support the actual message representation used by HighLevel.

Conceptually:

```text
event.data
    ↓
if string
    ↓
JSON.parse(...)
    ↓
object
```

while also supporting an already-object payload.

Do not blindly `JSON.parse()` arbitrary objects.

Handle malformed strings safely.

Do not crash the payment page because of an unrelated `postMessage`.

---

# STEP 4 — ACCEPT THE ACTUAL PAYMENT CONTEXT

The message handler must recognize the same event types as the working implementation.

At minimum inspect and reproduce support for:

```text
payment_initiate_props
setup_initiate_props
```

where applicable to the existing implementation.

When the event is:

```text
payment_initiate_props
```

capture the actual:

```text
transactionId
orderId
amount
currency
contact
locationId
```

and any other fields explicitly provided by the working implementation.

Do NOT invent fields.

Do NOT fabricate a transaction ID.

Do NOT convert `orderId` into `transactionId`.

Do NOT generate UUIDs.

---

# STEP 5 — TRANSACTION ID MUST COME FROM REAL GHL CONTEXT

The primary transaction identifier for this payment flow should be the actual HighLevel transaction ID supplied by the existing payment integration.

Preferred source:

```text
payment_initiate_props.transactionId
```

because the working implementation demonstrates that this is the actual GHL payment context mechanism.

If the existing application also supports a legitimate transaction ID URL/query parameter, it may remain as a fallback.

However:

```text
payment_initiate_props
```

must no longer be treated as permanently suspended.

The implementation must be capable of receiving and using the real event.

If there is no real transaction ID after the supported context sources have been exhausted, fail safely:

```text
Invalid payment request. Missing payment transaction information.
```

Do not initiate a payment without a real transaction ID if the backend contract requires it.

---

# STEP 6 — CRITICAL: INCLUDE TRANSACTION ID IN PAYMENT INITIATION

The current `initiatePayment()` is incomplete.

The current request body is effectively:

```text
amount
paymentType
payerPhoneNumber
provider
clientId?
customerId?
merchantId?
```

It does NOT currently include the GHL transaction ID.

This must be corrected.

Before editing the payload, inspect the actual RVPay backend contract.

Search the repository for:

```text
CreateDepositRequest
transactionId
ghlTransactionId
deposit
CreateDeposit
public/deposits
```

Determine the exact field name expected by the existing backend.

If the backend contract expects:

```text
transactionId
```

use that.

If it expects another established field such as:

```text
ghlTransactionId
```

use the actual contract.

Do NOT guess.

Do NOT create a new backend endpoint.

Do NOT create a new field merely because it seems convenient.

The real GHL transaction ID must be passed through the existing supported contract.

---

# STEP 7 — DO NOT CONFUSE THE TWO IDENTIFIERS

Be extremely careful about:

```text
GHL transactionId
```

versus:

```text
RVPay/pawaPay deposit ID
```

They are not necessarily the same thing.

The GHL transaction ID comes from the HighLevel payment context.

The backend may return an RVPay deposit ID.

The current code correctly recognizes that the backend deposit ID is returned from:

```text
data.deposit.id
```

Do not replace the GHL transaction ID with that deposit ID.

The flow should preserve both concepts if the existing backend requires both:

```text
GHL transactionId
        ↓
RVPay backend correlation
        ↓
RVPay deposit ID
        ↓
provider transaction
```

Use the backend's existing correlation model.

---

# STEP 8 — PRESERVE THE EXISTING RVPay BACKEND

Do NOT redesign the backend.

Do NOT modify the Pawapay SDK.

Do NOT create another microservice.

Do NOT create:

```text
/payments/custom-provider/initiate
```

or any similar endpoint unless that endpoint already exists.

The current known endpoint is:

```text
POST /v1/public/deposits
```

but verify the CURRENT backend contract before finalizing.

Use the endpoint already implemented by RVPay.

The frontend should remain a client of the existing backend.

---

# STEP 9 — PRESERVE THE EXISTING PAYMENT INFORMATION

Keep the existing UI.

Do NOT redesign the page.

Preserve:

* amount
* country
* provider
* phone
* terms checkbox
* payment button
* status message
* provider selection
* country selection

The request should continue to map:

```text
country + phone
```

to the actual international phone number expected by the backend.

Continue using the existing provider mapping.

Continue using the existing currency mapping.

Do not add fake providers.

Do not modify the provider list unless required by the actual backend contract.

---

# STEP 10 — USE GHL CONTEXT FOR PREFILLING

The working implementation receives:

```text
amount
currency
contact
```

from HighLevel.

The current page already contains logic intended to consume:

```text
payment_initiate_props
```

Do not delete that useful behavior.

Once the handshake is repaired, ensure that:

```text
amount
currency
contact.phone
```

can actually populate the existing UI.

Preserve the existing fallback behavior where legitimate URL parameters are supported.

Do not allow malformed GHL context to crash the page.

---

# STEP 11 — STATUS POLLING

After payment initiation succeeds:

```text
POST initiation
       ↓
backend accepts request
       ↓
poll existing status endpoint
```

Do not immediately declare payment success.

Use the existing:

```text
GET /v1/public/payments/verify
```

contract if that is still the current backend endpoint.

Pass the appropriate real transaction correlation identifier according to the backend contract.

Do not fabricate identifiers.

Do not start multiple polling intervals.

Do not allow duplicate payment submissions while one is processing.

Always clean up polling on:

* success
* failure
* error
* unmount
* timeout, if the existing implementation has a timeout contract

---

# STEP 12 — DO NOT BREAK THE IFRAME

Keep the useful diagnostics.

Continue logging useful information such as:

```text
[RVPay] Payment iframe initialized
[RVPay] window === window.parent
[RVPay] current origin
[RVPay] parent origin
[RVPay] Sending ready event
[RVPay] Ready event sent
[RVPay] MESSAGE RECEIVED
[RVPay] event.origin
[RVPay] event.source === window.parent
[RVPay] event.data
```

Also log clearly when:

```text
payment_initiate_props
```

is received.

For example:

```text
[RVPay] payment_initiate_props RECEIVED
[RVPay] transactionId:
[RVPay] orderId:
[RVPay] amount:
[RVPay] currency:
[RVPay] locationId:
```

Do not log secrets or credentials.

---

# STEP 13 — ORIGIN / SECURITY

Do not blindly trust arbitrary origins beyond what is necessary for the existing HighLevel integration.

Inspect how the working implementation handles:

```text
event.origin
event.source
```

Preserve the established security behavior where possible.

Do not expose:

* client secrets
* provider credentials
* access tokens
* private API keys

Do not put server-side credentials into `NEXT_PUBLIC_*`.

---

# STEP 14 — NEXT.JS COMPATIBILITY

The page is a client component.

Use the Next.js approach already established by this repository.

Avoid SSR/build-time access to browser-only APIs.

If using:

```text
window
document
postMessage
URLSearchParams
```

keep browser-only operations inside appropriate client-side lifecycle code.

Do not introduce unnecessary dependencies.

Do not introduce a hydration mismatch.

---

# STEP 15 — DO NOT MAKE URL PARAMETERS THE PRIMARY THEORY

The previous implementation attempted to treat:

```text
?transactionId=...
```

as the authoritative mechanism.

Do not assume this is correct simply because the URL may contain other payment parameters.

The working `/payment/checkout` implementation demonstrates that HighLevel supplies payment context through the iframe message protocol.

Therefore the comparison must determine the actual authoritative source from the working implementation.

Do not use:

```text
buyNowProductId
```

as a transaction ID.

Do not use:

```text
orderId
```

as a transaction ID.

Do not generate one.

---

# STEP 16 — TEST THE ACTUAL MESSAGE PATH

Before declaring the task complete, trace the code path:

```text
Payment page loads
      ↓
message listener registered
      ↓
custom_provider_ready sent
      ↓
ready retry mechanism operates
      ↓
HighLevel responds
      ↓
event.data parsed
      ↓
payment_initiate_props recognized
      ↓
transactionId captured
      ↓
amount/currency/contact captured
      ↓
Pay button enabled/usable
      ↓
initiatePayment()
      ↓
existing RVPay backend endpoint
      ↓
real transaction context included
      ↓
backend response
      ↓
existing status polling
```

The critical point is that the page must no longer fail because HighLevel's response happens to arrive as a JSON string.

---

# STEP 17 — DO NOT MODIFY UNRELATED FILES

Prefer changing only:

```text
admindashboard/app/payment/page.tsx
```

If the working implementation proves that one directly-related client/helper file must be changed, stop and explain why before making broad changes.

Do NOT modify:

* Pawapay SDK
* backend microservices
* Docker architecture
* database
* unrelated admin pages
* authentication
* infrastructure
* unrelated API contracts

unless the repository proves that a minimal change is absolutely required.

---

# ACCEPTANCE CRITERIA

Do not report completion unless all of the following are satisfied.

### A. Working implementation was actually inspected

Cline identifies the exact working `/payment/checkout` implementation and explains the relevant differences.

### B. Handshake

The current `/payment` page reproduces the working GHL ready-message behavior.

### C. Retry

The ready message is retried according to the proven working implementation.

### D. Message representation

The listener supports the actual message representation used by the working implementation, including JSON-stringified messages if applicable.

### E. Event

The page recognizes:

```text
payment_initiate_props
```

and the other event type(s) supported by the working implementation where appropriate.

### F. Transaction ID

The page obtains the REAL HighLevel transaction ID.

No:

```text
UUID
random ID
orderId substitution
fabricated transactionId
```

### G. Payment initiation

The Pay button actually invokes the existing RVPay payment initiation endpoint.

### H. Transaction correlation

The REAL GHL transaction ID is included in the request using the backend's ACTUAL expected field/contract.

### I. Backend

No invented backend endpoint.

### J. Status

After initiation, the existing status endpoint is polled using the correct real correlation identifier.

### K. Duplicate prevention

Repeated Pay clicks cannot create duplicate requests while processing.

### L. UI

Existing payment UI remains intact.

### M. Security

No secrets are exposed.

### N. Build

Run the repository's existing:

* TypeScript validation
* lint, if configured
* production Next.js build

Use the project's actual commands.

### O. Live limitation

Clearly distinguish:

```text
verified by source/code inspection
verified by local build
```

from:

```text
verified against a live HighLevel payment iframe
```

Do not claim live HighLevel success unless it was actually tested.

---

# REQUIRED FINAL REPORT

When finished, report:

1. Exact file(s) changed.

2. Exact working implementation inspected.

3. The critical difference between the old `/payment` implementation and the working `/payment/checkout` implementation.

4. How the GHL ready handshake now works.

5. How incoming `payment_initiate_props` is parsed.

6. How `transactionId` is obtained.

7. Exact existing RVPay backend endpoint used for initiation.

8. The high-level initiation payload, explicitly showing where the real GHL transaction ID goes.

9. Which existing endpoint handles status polling.

10. Whether `payment_initiate_props` is now authoritative/usable.

11. Build/lint/typecheck results.

12. Any remaining limitation.

13. Explicitly distinguish code/build verification from live HighLevel verification.

The final report must demonstrate the complete code path:

```text
HighLevel
   ↓
custom_provider_ready
   ↓
payment_initiate_props
   ↓
REAL transactionId
   ↓
Pay button
   ↓
existing RVPay initiation API
   ↓
backend
   ↓
deposit/provider flow
   ↓
existing status endpoint
   ↓
payment result
```

Do not claim this is fixed merely because the page compiles.

Proceed by inspecting the CURRENT repository and the known-working implementation first.

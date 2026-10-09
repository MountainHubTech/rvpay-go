# Agent 02 — PawaPay Deposits

## Goal
Connect Transactions deposits to the current PawaPay client.

## Read
- `00-pawapay-context.md`
- `transactions/.service-context.md`
- `transactions/.service-checkpoint.md`
- `deposits/deposits/*`
- `transactions/deposits/*`
- `transactions/deposits/merchants/*`
- `transactions/deposits/payments/*`
- current PawaPay deposit API

## Tasks
- Identify the Transactions deposit execution point.
- Follow the legacy Deposits implementation pattern.
- Use only current PawaPay deposit methods.
- Preserve existing Transactions contracts.
- Pass the correct request data.
- Follow existing Transactions error handling.
- Do not add reconciliation, callbacks, webhooks, retries, or new provider abstractions.

## Tests
Test successful provider calls, provider errors, validation failures, and no provider call on invalid input.

## Completion
Update `transactions/.clinecheck.md`, `transactions/.service-context.md`, `transactions/.service-checkpoint.md`.

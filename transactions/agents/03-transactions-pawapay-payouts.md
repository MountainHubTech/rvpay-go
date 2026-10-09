# Agent 03 — PawaPay Payouts

## Goal
Connect Transactions payouts to the current PawaPay client.

## Read
- `00-pawapay-context.md`
- `transactions/.service-context.md`
- `transactions/.service-checkpoint.md`
- `transactions/payouts/*`
- current PawaPay payout API

## Tasks
- Identify the Transactions payout execution point.
- Follow existing Transactions style and legacy provider injection.
- Use only current PawaPay payout methods.
- Preserve current domain and protobuf contracts.
- Propagate provider errors using existing conventions.
- Do not add future payout functionality.
- Do not invent request or response types.

## Tests
Test successful payout calls, provider errors, validation failures, and provider wiring.

## Completion
Update `transactions/.clinecheck.md`, `transactions/.service-context.md`, `transactions/.service-checkpoint.md`.

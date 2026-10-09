# Agent 00 — PawaPay Context

## Goal
Add the existing `pawapay_client` to the Transactions service.

## Scope
- Work only under `transactions/`.
- Use the legacy `deposits/` service as the implementation guide.
- Use only methods currently present in `pawapay_client`.
- Do not invent SDK types, constructors, interfaces, or request syntax.
- Do not modify `pawapay_client`.
- Do not refactor unrelated Transactions code.

## Environment
Only these new PawaPay variables:
- `PAWAPAY_API_URL`
- `PAWAPAY_API_KEY`

## Required work
1. Read this file.
2. Read `transactions/.service-context.md`, `transactions/.service-checkpoint.md`, `transactions/.clinecheck.md`.
3. Inspect legacy PawaPay wiring.
4. Inspect the current PawaPay client API. You can find the client at https://github.com/I-Frostbyte/pawapay_client
5. Integrate the client into Transactions.
6. Integrate available deposit and payout methods.
7. Add focused tests.
8. Update tracking files after every task.
9. Append the final integration details to `transactions/README.md`.

## Guardrails
Keep changes small.
Do not read large unrelated directories.
Do not redesign Transactions.
Do not add future PawaPay functionality.
Tests decide completion.

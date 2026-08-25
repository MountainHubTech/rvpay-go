# Agent 01 — PawaPay Bootstrap

## Goal
Wire the existing PawaPay client into Transactions.

## Read
- `00-pawapay-context.md`
- `transactions/.service-context.md`
- `transactions/.service-checkpoint.md`
- `deposits/cmd/grpc-service/main.go`
- `deposits/config/`
- `deposits/deposits/`
- `transactions/cmd/grpc-service/main.go`
- `transactions/config/`
- current PawaPay client README/API

## Tasks
- Add the PawaPay dependency using the existing Go module pattern.
- Add only `PAWAPAY_API_URL` and `PAWAPAY_API_KEY`.
- Follow the legacy Deposits configuration/client construction pattern.
- Inject the client into Transactions deposit and payout layers.
- Keep the existing server lifecycle intact.
- Do not create new abstractions unless the existing pattern requires one.

## Tests
Test configuration loading and PawaPay wiring.
Run focused Transactions tests.

## Completion
Update `transactions/.clinecheck.md`, `transactions/.service-context.md`, `transactions/.service-checkpoint.md`.

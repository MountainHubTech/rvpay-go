# Agent 04 — Tests and Verification

## Goal
Verify the complete Transactions PawaPay integration.

## Read
- `00-pawapay-context.md`
- `transactions/.service-context.md`
- `transactions/.service-checkpoint.md`
- `.clinecheck.md`
- files changed by previous agents

## Tasks
- Verify every PawaPay call uses a current SDK method.
- Verify only two new PawaPay variables exist.
- Verify client wiring through the Transactions bootstrap.
- Verify deposit and payout tests.
- Add only missing focused tests.
- Review for unrelated changes.

## Commands
Start with:
- `go test ./transactions/...`
- `go vet ./transactions/...`
- `gofmt` on changed Go files

Run `go test ./...` if practical.

## Completion
Update `transactions/.clinecheck.md`, `transactions/.service-context.md`, `transactions/.service-checkpoint.md`.

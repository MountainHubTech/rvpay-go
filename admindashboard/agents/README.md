# Rudimentary Dashboard — Cline Dockerization Agents

These prompts are designed for a sequential, low-context Cline workflow.

## Sequence

1. `00-bootstrap.md`
2. `01-recon.md`
3. `02-next-production.md`
4. `03-docker.md`
5. `04-env-workflow.md`
6. `05-makefile-aws-interface.md`
7. `06-final-review.md`

Agent 00 creates the repository control-plane files used by the remaining agents. Each subsequent agent reads the checkpoint and project context rather than repeatedly scanning the whole repository.

The agents deliberately avoid AWS infrastructure implementation. Their goal is a small, reproducible production container plus the operational interface needed for a later AWS deployment decision.

## Design goals

- low Cline token/context usage
- sequential handoffs
- minimal repository rereading
- no unnecessary dependency changes
- no application redesign
- no premature AWS infrastructure
- explicit viability checks without excessive testing

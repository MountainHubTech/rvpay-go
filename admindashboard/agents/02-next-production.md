# Agent 02 — Next.js Production Configuration

## Mission

Apply only the minimum Next.js configuration required to make the application suitable for the Docker runtime identified by Agent 01.

## Read first

1. `.clinerules.md`
2. `.project-context.md`
3. `.project-checkpoint.md` — Agent 01 completion and Task 2 reading map
4. `AGENTS.md`
5. `next.config.ts`

Read `package.json` only if the checkpoint does not contain the required version/script facts.

Before changing Next.js configuration, follow the repository's `AGENTS.md` instruction to consult the relevant installed Next.js 16 documentation when available.

## Tasks

1. Review Agent 01's findings.
2. Determine the smallest configuration change required for the chosen production runtime.
3. Apply that change to `next.config.ts` only if justified.
4. Preserve all existing behavior.
5. Do not add unrelated optimization flags.
6. Do not add experimental features unless required and verified.

## Viability checks

Run the smallest useful validation:

- configuration/type validation
- production build if practical and not excessively expensive

If a build is unavailable because dependencies are missing, record that fact rather than installing unnecessary tooling.

## Constraints

- Do not create Dockerfile yet.
- Do not create Makefile yet.
- Do not modify application components.
- Do not change dependencies unless Agent 01 proved it is strictly necessary.
- Do not perform dependency upgrades.
- Do not introduce AWS code.

## Output

Update `.project-context.md` with the final Next.js production configuration decision.

Append `.project-checkpoint.md` with:

- files changed
- validation performed
- result
- exact Docker requirements for Agent 03
- unresolved issues

Update `.clinecheck.md` only for verified checks.

## Handoff

Agent 03 should be able to build the Dockerfile without rereading the application source tree.

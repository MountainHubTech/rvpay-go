# Agent 01 — Containerization Reconnaissance

## Mission

Determine the minimum technical requirements for running the existing Next.js application as a production Docker container.

Do not implement Dockerization.

## Read first

Read only:

1. `.clinerules.md`
2. `.project-context.md`
3. `.project-checkpoint.md` — Agent 00 section and Task 1 reading map
4. `AGENTS.md`
5. `package.json`
6. `next.config.ts`
7. `tsconfig.json`
8. `README.md` — only the Dockerization section if the checkpoint says it is sufficient

Read additional source files only when required to determine environment-variable use, server/runtime behavior, or build requirements.

## Tasks

1. Confirm package manager and lockfile.
2. Confirm Node/Next.js requirements from the repository and installed dependency metadata where available.
3. Determine whether `next build` produces a server runtime suitable for a container.
4. Determine whether Next.js standalone output is appropriate.
5. Find environment-variable references:
   - server-only
   - client-exposed (`NEXT_PUBLIC_*`)
   - build-time-sensitive
   - runtime-sensitive
6. Determine the application listening-port expectations.
7. Identify any static files, fonts, native modules, or special runtime requirements that affect the image.
8. Identify anything that would make a standard multi-stage Next.js image unsuitable.
9. Record only facts relevant to Dockerization.

## Viability checks

Run only lightweight checks that materially answer the above.

Prefer existing package/config inspection.

If dependencies are not installed, do not install them solely for curiosity. Record what cannot be verified.

## Constraints

- Do not modify application source.
- Do not modify `next.config.ts`.
- Do not create Docker files.
- Do not create a Makefile.
- Do not upgrade dependencies.
- Do not redesign the application.
- Do not invent AWS infrastructure.

## Output

Update `.project-context.md` only with stable facts discovered by this agent.

Append to `.project-checkpoint.md`:

- Agent 01 completion record
- findings
- viability checks
- decisions
- unresolved items
- exact handoff to Agent 02

Update `.clinecheck.md` only for checks actually completed.

## Handoff

Agent 02 must receive a minimal, explicit statement of what Next.js configuration changes, if any, are required before Dockerization.

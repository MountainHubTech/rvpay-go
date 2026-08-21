# Agent 00 — Cline Bootstrap & Project Control Plane

## Mission

Prepare the repository for the Dockerization agent sequence by creating and maintaining the lightweight project-control files used by Agents 01–06.

This agent does **not** dockerize the application. It establishes the shared context, constraints, checkpoint protocol, reading map, and completion criteria that keep later Cline runs small and consistent.

## Repository

`I-Frostbyte/rudimentary-dashboard`

Current application facts already established:

- Next.js `16.3.0`
- React `19.2.8`
- npm with committed `package-lock.json`
- TypeScript
- Tailwind CSS 4
- ESLint 9
- Existing root files include `AGENTS.md`, `CLAUDE.md`, `README.md`, `next.config.ts`, `package.json`, `package-lock.json`, `tsconfig.json`, and `eslint.config.mjs`
- Existing `README.md` is the default create-next-app README and must be appended to, not replaced.
- `next.config.ts` currently contains no meaningful production configuration.
- `AGENTS.md` explicitly requires checking the installed Next.js 16 documentation before writing Next.js code.

Do not assume anything beyond the repository. Verify only what is necessary.

## Files to create/update

Create:

- `.clinerules.md`
- `.clineignore.md`
- `.project-context.md`
- `.project-checkpoint.md`
- `.clinecheck.md`
- `.project-next-steps`

Update:

- `README.md` by appending a short section explaining the new agent-controlled Dockerization workflow. Do not rewrite or remove the existing README.

Do not create Docker files, Makefile, AWS infrastructure, CI/CD, or application refactors.

## Operating principles

1. Read narrowly. Do not recursively inspect the whole repository unless a required fact cannot be established otherwise.
2. Prefer package/config metadata over source-code exploration.
3. Never invent environment variables, AWS resources, ports, commands, or deployment architecture.
4. Preserve the existing application.
5. Dockerization is the goal; modernization is not.
6. Keep later agents independent and cheap by recording facts once.
7. If a fact is unknown, record `UNKNOWN — verify in Agent N` rather than guessing.
8. Do not install packages merely to investigate.
9. Do not modify application source code in this agent.
10. Do not make unrelated cleanup changes.

## `.project-context.md`

Record stable facts that constrain all later agents.

Include:

- project identity and purpose as currently observable
- framework/runtime versions
- package manager and lockfile
- build/start/lint commands
- important configuration files
- current Next.js configuration state
- environment-variable findings
- runtime/build assumptions
- Docker constraints
- AWS constraints that are already known
- forbidden/unapproved additions
- decisions that later agents must not revisit
- facts that still require verification

Separate **FACT**, **DECISION**, and **UNKNOWN** entries.

Do not paste source files into this document.

## `.clineignore.md`

List repository areas/files that later agents should avoid reading unless directly required.

At minimum consider:

- `node_modules/`
- `.next/`
- build/output directories
- coverage
- local environment files
- generated caches
- `.git/`

Also state that agents should avoid broad source-tree rereads when the checkpoint and context already answer the question.

This is a project guidance file. Do not assume Cline will treat the `.md` filename as its native ignore mechanism.

## `.project-checkpoint.md`

Initialize a chronological checkpoint ledger.

Create:

1. Project baseline
2. Reading map
3. Agent 00 completion record
4. Empty sections for Agents 01–06

The reading map must be task-specific. Use a table such as:

| Task | Agent | Required reading | Optional reading | Do not reread unless blocked |
|---|---|---|---|---|

The initial map should be conservative. Agents should read only the files needed for their task.

Each future agent must append its result rather than rewriting prior records.

Each completion record should contain:

- agent
- date
- status
- files changed
- checks run
- findings
- decisions
- unresolved issues
- next-agent handoff
- exact additional reading required, if any

## `.clinecheck.md`

Create a lightweight verification checklist for the entire sequence.

Include gates for:

- bootstrap complete
- application configuration valid
- Docker build valid
- production container starts
- runtime environment handling verified
- Makefile workflow valid
- production image is appropriately minimal
- no secrets copied into the image
- final AWS-readiness assessment complete

Use checkboxes and keep this file concise.

Agents may mark only checks they actually verified.

## `.project-next-steps`

Create the file with a clear status of:

`NOT READY — Dockerization sequence has not completed.`

Reserve it for Agent 06.

Agent 00 must not write AWS deployment instructions into it.

## README.md

Append only a small section:

- explain that Dockerization is being performed through the numbered Cline agents
- point agents/developers to the project-control files
- state that AWS deployment steps are documented only after Agent 06 completes

Do not replace the existing README.

## Verification

Before finishing:

- confirm all six new project-control files exist
- confirm README was appended rather than replaced
- inspect the resulting files for contradictions
- run only lightweight checks needed to validate the created control files
- do not build Docker
- do not change application code

## Completion

Append the Agent 00 completion record to `.project-checkpoint.md`.

End with a concise handoff naming exactly what Agent 01 must read.

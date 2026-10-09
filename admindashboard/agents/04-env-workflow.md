# Agent 04 — Environment Handling & Local Docker Workflow

## Mission

Make configuration handling safe and predictable for local Docker execution and future AWS deployment.

## Read first

1. `.clinerules.md`
2. `.project-context.md`
3. `.project-checkpoint.md` — Agent 01–03 records and Task 4 reading map
4. `package.json`
5. `.gitignore`
6. `Dockerfile`
7. `.dockerignore`

Read source files only for environment variables explicitly identified as unresolved.

## Tasks

1. Confirm every environment variable actually used by the application.
2. Classify variables as:
   - public/client
   - server/runtime
   - build-time-sensitive
3. Ensure secrets are not copied into the Docker image.
4. Establish the smallest useful local runtime environment workflow.
5. Add/update an example environment file only if justified by actual variables.
6. Document how runtime environment values are supplied to the container.
7. Ensure local development behavior remains unchanged.

## Important

Never invent values for secrets.

Never commit real credentials.

Do not convert runtime secrets into `NEXT_PUBLIC_*`.

Do not bake environment-specific values into the Docker image unless Next.js genuinely requires them at build time and the project context explicitly records that decision.

## Viability checks

Run a basic container startup check using non-secret configuration where possible.

If a required secret prevents a full application check, verify container startup and document the limitation.

Do not add external secret-management software in this task.

## Constraints

- Do not create AWS infrastructure.
- Do not create a CI/CD pipeline.
- Do not introduce Docker Compose unless required.
- Do not refactor application configuration unnecessarily.

## Output

Update `.project-context.md` with the final environment model.

Append `.project-checkpoint.md` with:

- variables discovered
- classification
- files changed
- checks
- limitations
- exact requirements for Agent 05

Update `.clinecheck.md` only for verified environment checks.

# Agent 03 — Production Docker Image

## Mission

Create the smallest practical production Docker image for the existing Next.js application.

## Read first

1. `.clinerules.md`
2. `.project-context.md`
3. `.project-checkpoint.md` — Agent 01 and Agent 02 records plus Task 3 reading map
4. `package.json`
5. `next.config.ts`

Do not reread the application source unless the checkpoint identifies an unresolved runtime requirement.

## Tasks

Create only the Docker artifacts justified by the project:

- `Dockerfile`
- `.dockerignore`

Use a multi-stage build.

Target principles:

- deterministic dependency installation using the committed lockfile
- build dependencies isolated from runtime
- minimal runtime filesystem
- production-only runtime artifacts
- no source tree unless required
- no package manager cache in the final image
- non-root runtime where compatible
- correct port and host binding
- correct Next.js production entrypoint
- no secrets in image layers

Use a current, stable Node image compatible with the repository's Next.js version. Do not guess a version if the project context contains a verified requirement.

## Viability checks

Build the image.

If practical, start the image and make a basic HTTP request.

Do not run an expensive battery of tests.

If a check fails, fix only Docker/runtime issues introduced by this task.

## Constraints

- No Docker Compose unless proven necessary.
- No Kubernetes.
- No CI/CD.
- No AWS infrastructure.
- No application refactor.
- No dependency upgrade.
- No unrelated formatting.
- Do not add healthcheck tooling that requires extra packages unless necessary.
- Do not optimize prematurely at the expense of clarity.

## Image review

Inspect the final image enough to establish:

- it contains only expected runtime artifacts
- it does not contain local `.env*` files or obvious secrets
- it does not contain development source/dependency material unnecessarily
- it starts with the expected command

Record image size if available.

## Output

Append `.project-checkpoint.md` with the Docker implementation, checks, image findings, and Agent 04 handoff.

Update `.project-context.md` with only stable Docker decisions.

Update `.clinecheck.md` only for checks actually completed.

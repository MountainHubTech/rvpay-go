# Agent 05 — Makefile & AWS Container Interface

## Mission

Create a small Makefile that makes the Docker workflow repeatable and prepares a clean interface for eventual AWS deployment.

This is an interface layer, not the AWS deployment implementation.

## Read first

1. `.clinerules.md`
2. `.project-context.md`
3. `.project-checkpoint.md` — Agents 01–04 and Task 5 reading map
4. `Dockerfile`
5. `.dockerignore`
6. `package.json`
7. `.gitignore`

Do not reread application components.

## Tasks

Create a root `Makefile` with concise targets appropriate to the repository.

At minimum consider:

- help
- local application validation/build
- Docker image build
- Docker run
- Docker stop/cleanup if useful
- Docker verification
- image tagging
- an ECR login/push interface only if it can be kept environment-agnostic

Use variables rather than hardcoding AWS account IDs, regions, repositories, credentials, or production values.

Make targets should fail clearly when required variables are absent.

Keep AWS-related targets as a thin interface. Do not implement AWS infrastructure here.

## Cost/minimalism rules

- No Docker Compose unless already required.
- No Terraform/CDK/CloudFormation.
- No deployment scripts that duplicate Makefile logic.
- No shell framework.
- No dependency installation.
- No unrelated Make targets.
- Prefer standard Docker and AWS CLI commands.

## Environment model

Respect the model established by Agent 04.

Never put secrets in the Makefile.

Do not hardcode production/test credentials.

## Viability checks

Run:

- `make help`
- the Docker build target
- the most useful local verification target

Run AWS CLI commands only if credentials/configuration are already available and doing so is safe. Otherwise validate command construction without pretending AWS deployment succeeded.

## Output

Append `.project-checkpoint.md` with:

- Makefile targets
- checks run
- AWS interface assumptions
- remaining AWS deployment work
- Agent 06 handoff

Update `.project-context.md` only with stable operational decisions.

Update `.clinecheck.md` only for verified checks.

# Agent 06 — Final Production Verification & AWS Readiness

## Mission

Independently verify that the application is now a usable production Docker service and document exactly what remains for AWS deployment.

This agent is allowed to fix small Dockerization defects discovered during verification.

## Read first

1. `.clinerules.md`
2. `.project-context.md`
3. `.project-checkpoint.md` — all prior completion records and Task 6 reading map
4. `Dockerfile`
5. `.dockerignore`
6. `Makefile`
7. `next.config.ts`
8. `package.json`
9. `README.md` — Dockerization section only

Do not reread the application source unless a verification failure requires it.

## Verification

Perform a concise production-path check:

1. install/build path is valid
2. Docker image builds
3. production container starts
4. application responds on the expected port
5. static assets/routes behave sufficiently for a smoke test
6. runtime environment handling matches the documented model
7. no obvious secrets are in the image
8. runtime image is appropriately minimal
9. non-root execution works if configured
10. Makefile Docker workflow works

Do not run a giant test suite merely for completeness.

## Fix policy

If a defect is found:

- fix only what is necessary
- preserve application behavior
- avoid dependency upgrades
- avoid architectural expansion
- rerun the affected check

If the problem is outside Dockerization scope, document it instead of expanding the project.

## AWS readiness assessment

Determine what is now true:

- image can be built reproducibly
- image can be tagged
- image can be pushed to a registry
- container can run with runtime environment values
- service can sit behind an AWS load balancer
- no AWS-specific application changes are required, if true

Then document what still needs to be decided/configured for AWS.

Do not create AWS infrastructure in this task.

Do not assume ECS, EC2, App Runner, EKS, or another service unless the repository/project context already specifies it.

## `.project-next-steps`

This is the only agent authorized to turn `.project-next-steps` into an AWS deployment guide.

Write:

1. Dockerization status
2. What is ready
3. Image/build workflow
4. Environment/secrets requirements
5. AWS prerequisites
6. Recommended low-cost deployment sequence
7. Items requiring an explicit AWS architecture decision
8. Rollback/update considerations
9. Commands or Make targets already available
10. Things deliberately not implemented

Do not claim AWS deployment is complete.

## Final project records

Append the final Agent 06 review to `.project-checkpoint.md`.

Update `.clinecheck.md` so it reflects only checks that actually passed.

Update `README.md` by appending a concise Docker/AWS-readiness section. Do not rewrite existing content.

## Completion rule

Only mark the project as Dockerization-complete if the production image builds and the container starts successfully, or if a clearly documented external limitation prevents that verification.

The final response to the user should be concise and list:

- files changed
- verification performed
- final Docker image/run status
- AWS readiness status
- unresolved items

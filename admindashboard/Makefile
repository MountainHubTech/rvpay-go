# ============================================================================
# rvpay-dashboard - repeatable Docker workflow + thin AWS ECR interface
#
# This Makefile turns the verified Docker workflow (recorded in
# .project-context.md by Agent 04) into repeatable targets and exposes a thin,
# environment-agnostic AWS ECR interface for a later deployment agent (06).
#
# Application facts this relies on (do not re-derive):
#   - Next.js 16.3.0 standalone build, node:22-alpine images
#   - Container listens on port 3000 (Next.js standalone default)
#   - No environment variables are required at build time or runtime
#   - Local image name is rvpay/admin
#
# SECRETS POLICY:
#   No secrets, credentials, account IDs, or AWS regions are committed here.
#   AWS values default to empty and the AWS targets fail loudly until supplied.
# ============================================================================

# base image name (matches the Dockerfile workflow recorded by Agent 04)
IMAGE      ?= rvpay/admin

# tag applied to local builds
TAG        ?= latest

# Next.js standalone port AND host listen port
PORT       ?= 3000

# container name for run/stop targets
CONTAINER  ?= rvpay-dashboard

# URL used by the verify target's HTTP smoke check
VERIFY_URL ?= http://127.0.0.1:$(PORT)/

# --- AWS ECR interface (intentionally empty; thin interface) ---------------
# Required only by AWS targets (tag / ecr-login / ecr-push). Left empty by
# default so those targets fail clearly until the caller supplies all three.
AWS_REGION     ?=
AWS_ACCOUNT_ID ?=
AWS_ECR_REPO   ?=
AWS_REGISTRY   ?= $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com

# ============================================================================
# Targets
# ============================================================================

# list available targets and the AWS interface variables
help:
	@echo "rvpay-dashboard — available targets:"
	@echo "  build        build the Docker image as $(IMAGE):$(TAG)"
	@echo "  run          start a detached container on port $(PORT)"
	@echo "  stop         stop and remove the container"
	@echo "  verify       build, start, HTTP-check 200, then stop"
	@echo "  tag          tag the image for the AWS ECR registry"
	@echo "  ecr-login    obtain an AWS ECR login token"
	@echo "  ecr-push     push the tagged image to AWS ECR"
	@echo ""
	@echo "AWS interface variables (AWS targets require all three):"
	@echo "  AWS_REGION     e.g. us-east-1"
	@echo "  AWS_ACCOUNT_ID 12-digit AWS account id"
	@echo "  AWS_ECR_REPO   repository name"

# --- local Docker workflow ---------------------------------------------------

# build the Docker image (full application build lives in the Dockerfile)
build:
	docker build -t "$(IMAGE):$(TAG)" .

# run a detached container and report its URL
run:
	docker run -d -p "$(PORT):3000" --name "$(CONTAINER)" "$(IMAGE):$(TAG)"
	@echo "container running at http://localhost:$(PORT)/"
	@echo "stop it later with: make stop"

# stop and remove the container; idempotent
stop:
	@-docker stop "$(CONTAINER)" 2>/dev/null || true
	@-docker rm "$(CONTAINER)" 2>/dev/null || true

# most useful local verification: build, start, expect HTTP 200, then stop
verify: build
	@docker run -d -p "$(PORT):3000" --name "$(CONTAINER)" "$(IMAGE):$(TAG)"
	@code=""; \
	for i in 1 2 3 4 5 6 7 8 9 10 11 12; do \
		code=$$(curl -s -o /dev/null -w "%{http_code}" "$(VERIFY_URL)" 2>/dev/null); \
		if [ "$$code" = "200" ]; then break; fi; \
		sleep 1; \
	done; \
	if [ "$$code" = "200" ]; then \
		echo "PASS — $(VERIFY_URL) returned HTTP 200"; \
	else \
		echo "FAIL — expected HTTP 200, got [$$code]"; \
		docker stop "$(CONTAINER)" 2>/dev/null; \
		exit 1; \
	fi
	@-docker stop "$(CONTAINER)" 2>/dev/null || true
	@-docker rm "$(CONTAINER)" 2>/dev/null || true

# --- AWS ECR interface (thin; no AWS infrastructure is created here) ----------

# reusable guard so every AWS target fails clearly when required vars are absent
require_aws:
	@test -n "$(AWS_REGION)" && test -n "$(AWS_ACCOUNT_ID)" && test -n "$(AWS_ECR_REPO)" || { \
		echo "ERROR: AWS interface targets need AWS_REGION, AWS_ACCOUNT_ID, and AWS_ECR_REPO."; \
		echo "Supply them per-invocation, e.g."; \
		echo "  make ecr-push AWS_REGION=us-east-1 AWS_ACCOUNT_ID=123456789012 AWS_ECR_REPO=rvpay-dashboard"; \
		exit 1; \
	}

# tag the built image with the caller's ECR registry URI
tag: require_aws
	docker tag "$(IMAGE):$(TAG)" "$(AWS_REGISTRY)/$(AWS_ECR_REPO):$(TAG)"

# obtain an AWS ECR login token (the caller must already have AWS credentials)
ecr-login: require_aws
	@echo "region=$(AWS_REGION) repository=$(AWS_ECR_REPO)"
	aws ecr get-login-password --region "$(AWS_REGION)"

# push the tagged image to the caller's ECR repository
ecr-push: require_aws tag
	docker push "$(AWS_REGISTRY)/$(AWS_ECR_REPO):$(TAG)"
SHELL := /usr/bin/env bash

AWS_PROFILE ?= default
AWS_REGION ?= eu-central-1
ACM_REGION ?= us-east-1
GO_BIN ?= go
FRONTEND_DOMAIN ?= mandelbrot.i-dmytro.org
TF_BACKEND_CONFIG ?= backend.hcl
BACKEND_DIR := backend
FRONTEND_DIR := frontend/fractal-app
BUILD_DIR := build

.PHONY: help
help:
	@printf "Mandelbrot project commands\n\n"
	@printf "Build and test:\n"
	@printf "  make backend-test        Run Go backend tests\n"
	@printf "  make backend-local       Run local backend HTTP server in single mode\n"
	@printf "  make backend-local-distributed Run local backend HTTP server in orchestrator mode\n"
	@printf "  make local-api-smoke     Smoke-test local backend RGBA response\n"
	@printf "  make backend-build       Build Lambda bootstrap binary\n"
	@printf "  make backend-package     Build and zip Lambda package\n"
	@printf "  make frontend-install    Install frontend dependencies\n"
	@printf "  make frontend-local      Run local Vite frontend against local backend\n"
	@printf "  make frontend-build      Build frontend assets\n"
	@printf "  make frontend-lint       Run frontend lint\n\n"
	@printf "AWS and infra:\n"
	@printf "  make aws-whoami          Show active AWS identity\n"
	@printf "  make aws-inventory       Write docs/aws-inventory.md\n"
	@printf "  make aws-detail-inventory Write docs/aws-detail-inventory.md\n"
	@printf "  make old-infra-cleanup-plan Show old manual AWS cleanup actions\n"
	@printf "  make old-infra-cleanup  Delete old manual AWS infra\n"
	@printf "  make tf-state-init      Terraform init for remote-state bootstrap\n"
	@printf "  make tf-state-plan      Terraform plan for remote-state bootstrap\n"
	@printf "  make tf-state-apply     Terraform apply for remote-state bootstrap\n"
	@printf "  make tf-state-output    Show remote-state bootstrap outputs\n"
	@printf "  make tf-write-backend-config Write infra/terraform/backend.hcl\n"
	@printf "  make tf-migrate-state   Migrate app Terraform state to S3 backend\n"
	@printf "  make tf-reconfigure-backend Reconfigure app Terraform backend\n"
	@printf "  make tf-init             Terraform init in infra/terraform\n"
	@printf "  make tf-fmt              Format Terraform files\n"
	@printf "  make tf-validate         Validate Terraform configuration\n"
	@printf "  make tf-plan             Terraform plan in infra/terraform\n"
	@printf "  make tf-apply            Terraform apply in infra/terraform\n"
	@printf "  make tf-destroy          Terraform destroy in infra/terraform\n"
	@printf "  make tf-output           Show Terraform outputs\n\n"
	@printf "Deploy helpers:\n"
	@printf "  make deploy-frontend     Sync frontend dist to S3\n"
	@printf "  make invalidate-cdn      Invalidate CloudFront distribution\n"
	@printf "  make clean               Remove local build outputs\n\n"
	@printf "Common variables: AWS_PROFILE, AWS_REGION, ACM_REGION, FRONTEND_DOMAIN, GO_BIN, FRONTEND_BUCKET, DISTRIBUTION_ID\n"

.PHONY: backend-test
backend-test:
	cd $(BACKEND_DIR) && $(GO_BIN) test ./...

.PHONY: backend-local
backend-local:
	cd $(BACKEND_DIR) && MANDELBROT_LOCAL_HTTP_ADDR=:8080 MANDELBROT_HANDLER_MODE=single $(GO_BIN) run .

.PHONY: backend-local-distributed
backend-local-distributed:
	cd $(BACKEND_DIR) && MANDELBROT_LOCAL_HTTP_ADDR=:8080 MANDELBROT_HANDLER_MODE=orchestrator $(GO_BIN) run .

.PHONY: local-api-smoke
local-api-smoke:
	curl -fsS -o /tmp/mandelbrot-local.bin "http://localhost:8080/render?width=32&height_px=32&height=2.5&samples=1&maxIter=50&numBlocks=4&numThreads=2&tileSize=16"
	test "$$(wc -c < /tmp/mandelbrot-local.bin)" -eq 4096
	@printf "Local backend returned 4096 RGBA bytes\n"

.PHONY: backend-build
backend-build:
	GO_BIN=$(GO_BIN) ./scripts/package-backend.sh --build-only

.PHONY: backend-package
backend-package:
	GO_BIN=$(GO_BIN) ./scripts/package-backend.sh

.PHONY: frontend-install
frontend-install:
	cd $(FRONTEND_DIR) && npm install

.PHONY: frontend-local
frontend-local:
	cd $(FRONTEND_DIR) && VITE_API_URL=http://localhost:8080/render npm run dev -- --host 127.0.0.1

.PHONY: frontend-build
frontend-build:
	cd $(FRONTEND_DIR) && npm run build

.PHONY: frontend-lint
frontend-lint:
	cd $(FRONTEND_DIR) && npm run lint

.PHONY: aws-whoami
aws-whoami:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) ./scripts/aws-whoami.sh

.PHONY: aws-inventory
aws-inventory:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) ACM_REGION=$(ACM_REGION) ./scripts/aws-inventory.sh

.PHONY: aws-detail-inventory
aws-detail-inventory:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) ACM_REGION=$(ACM_REGION) ./scripts/aws-detail-inventory.sh

.PHONY: old-infra-cleanup-plan
old-infra-cleanup-plan:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) ./scripts/delete-old-manual-infra.sh

.PHONY: old-infra-cleanup
old-infra-cleanup:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) CONFIRM_DELETE_OLD_MANUAL_INFRA=true ./scripts/delete-old-manual-infra.sh

.PHONY: tf-state-init
tf-state-init:
	test -d infra/terraform-state || (echo "infra/terraform-state does not exist yet" && exit 1)
	cd infra/terraform-state && terraform init

.PHONY: tf-state-plan
tf-state-plan:
	test -d infra/terraform-state || (echo "infra/terraform-state does not exist yet" && exit 1)
	cd infra/terraform-state && terraform plan -var="aws_profile=$(AWS_PROFILE)" -var="aws_region=$(AWS_REGION)"

.PHONY: tf-state-apply
tf-state-apply:
	test -d infra/terraform-state || (echo "infra/terraform-state does not exist yet" && exit 1)
	cd infra/terraform-state && terraform apply -var="aws_profile=$(AWS_PROFILE)" -var="aws_region=$(AWS_REGION)"

.PHONY: tf-state-output
tf-state-output:
	test -d infra/terraform-state || (echo "infra/terraform-state does not exist yet" && exit 1)
	cd infra/terraform-state && terraform output

.PHONY: tf-write-backend-config
tf-write-backend-config:
	AWS_REGION=$(AWS_REGION) ./scripts/write-terraform-backend-config.sh

.PHONY: tf-migrate-state
tf-migrate-state:
	test -f infra/terraform/$(TF_BACKEND_CONFIG) || (echo "infra/terraform/$(TF_BACKEND_CONFIG) does not exist. Run make tf-write-backend-config first." && exit 1)
	cd infra/terraform && terraform init -migrate-state -force-copy -input=false -backend-config="$(TF_BACKEND_CONFIG)"

.PHONY: tf-reconfigure-backend
tf-reconfigure-backend:
	test -f infra/terraform/$(TF_BACKEND_CONFIG) || (echo "infra/terraform/$(TF_BACKEND_CONFIG) does not exist. Run make tf-write-backend-config first." && exit 1)
	cd infra/terraform && terraform init -reconfigure -input=false -backend-config="$(TF_BACKEND_CONFIG)"

.PHONY: tf-init
tf-init:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	test -f infra/terraform/$(TF_BACKEND_CONFIG) || (echo "infra/terraform/$(TF_BACKEND_CONFIG) does not exist. Run make tf-state-apply, then make tf-write-backend-config, then make tf-migrate-state." && exit 1)
	cd infra/terraform && terraform init -input=false -backend-config="$(TF_BACKEND_CONFIG)"

.PHONY: tf-fmt
tf-fmt:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	cd infra/terraform && terraform fmt

.PHONY: tf-validate
tf-validate:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	cd infra/terraform && terraform validate

.PHONY: tf-plan
tf-plan:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	cd infra/terraform && terraform plan -var="aws_profile=$(AWS_PROFILE)" -var="aws_region=$(AWS_REGION)" -var="acm_region=$(ACM_REGION)" -var="frontend_domain=$(FRONTEND_DOMAIN)"

.PHONY: tf-apply
tf-apply:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	cd infra/terraform && terraform apply -var="aws_profile=$(AWS_PROFILE)" -var="aws_region=$(AWS_REGION)" -var="acm_region=$(ACM_REGION)" -var="frontend_domain=$(FRONTEND_DOMAIN)"

.PHONY: tf-destroy
tf-destroy:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	cd infra/terraform && terraform destroy -var="aws_profile=$(AWS_PROFILE)" -var="aws_region=$(AWS_REGION)" -var="acm_region=$(ACM_REGION)" -var="frontend_domain=$(FRONTEND_DOMAIN)"

.PHONY: tf-output
tf-output:
	test -d infra/terraform || (echo "infra/terraform does not exist yet" && exit 1)
	cd infra/terraform && terraform output

.PHONY: deploy-frontend
deploy-frontend:
	AWS_PROFILE=$(AWS_PROFILE) AWS_REGION=$(AWS_REGION) FRONTEND_BUCKET=$(FRONTEND_BUCKET) DISTRIBUTION_ID=$(DISTRIBUTION_ID) ./scripts/deploy-frontend.sh

.PHONY: invalidate-cdn
invalidate-cdn:
	test -n "$(DISTRIBUTION_ID)" || (echo "DISTRIBUTION_ID is required" && exit 1)
	aws cloudfront create-invalidation --profile $(AWS_PROFILE) --distribution-id $(DISTRIBUTION_ID) --paths "/*"

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR) $(FRONTEND_DIR)/dist

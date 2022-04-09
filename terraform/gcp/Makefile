.PHONY: help
help: ## Print this help
help:
	@echo 'Usage: make <target>'
	@echo
	@echo -e 'Targets:'
	@egrep '^(.+)\:\ ##\ (.+)' $(MAKEFILE_LIST) | column -t -c 2 -s ':#'

.PHONY: packer/validate
packer/validate: ## Validate the HashiStack Packer configuration
	packer validate packer.json

.PHONY: packer/build
packer/build: ## Build the HashiStack image with Packer
	time packer build \
		-force \
		-timestamp-ui \
		-var="project=${GOOGLE_PROJECT}" \
		-var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" \
		packer.json

.PHONY: terraform/validate
terraform/validate: ## Validate the Terraform configuration
	cd modules/hashistack; \
	terraform init; \
	terraform validate .  

.PHONY: terraform/plan
terraform/plan: ## Run a Terraform Plan 
	cd modules/hashistack; \
	terraform init; \
	terraform plan \
		-var="project=${GOOGLE_PROJECT}" \
		-var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 

.PHONY: terraform/apply
terraform/apply: ## Run a Terraform Apply
	cd modules/hashistack; \
	terraform init; \
	terraform apply \
		-auto-approve \
		-var="project=${GOOGLE_PROJECT}" \
		-var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 

.PHONY: terraform/destroy
terraform/destroy: ## Plan the Terraform configuration
	cd modules/hashistack; \
	terraform init; \
	terraform destroy \
		-auto-approve \
		-var="project=${GOOGLE_PROJECT}" \
		-var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 

.PHONY: terraform/validate/example/us-east
terraform/validate/example/us-east: ## Validate the Terraform configuration for the us-east example env 
	cd env/us-east; \
	terraform init; \
	terraform validate .  

.PHONY: terraform/plan/example/us-east
terraform/plan/example/us-east: ## Run a Terraform Plan for the us-east example env 
	cd env/us-east; \
	terraform init; \
	terraform plan -var="project=${GOOGLE_PROJECT}" -var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 

.PHONY: terraform/apply/example/us-east
terraform/apply/example/us-east: ## Run a Terraform Apply for the us-east example env 
	cd env/us-east; \
	terraform init; \
	terraform apply \
		-auto-approve \
		-var="project=${GOOGLE_PROJECT}" \
		-var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}" 

.PHONY: terraform/destroy/example/us-east
terraform/destroy/example/us-east: ## Plan the Terraform configuration for the us-east example env 
	cd env/us-east; \
	terraform init; \
	terraform destroy \
		-auto-approve \
		-var="project=${GOOGLE_PROJECT}" \
		-var="credentials=${GOOGLE_APPLICATION_CREDENTIALS}"

.PHONY: gcloud/login/server
gcloud/login/server: ## Login to a server instance using gcloud
	gcloud compute ssh hashistack-server-0

.PHONY: gcloud/login/client
gcloud/login/client: ## Login to a client instance using gcloud
	gcloud compute ssh hashistack-client-0

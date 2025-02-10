PKG_PATH ?= $(shell pwd)/../../pkg/linux_amd64/nomad
NOMAD_LICENSE_PATH ?=
CONSUL_LICENSE_PATH ?=

# deploy for quick local development testing

custom.tfvars:
	echo 'nomad_local_binary = "$(PKG_PATH)"' > custom.tfvars
	echo 'volumes = false' >> custom.tfvars
	echo 'client_count_linux = 3' >> custom.tfvars
	echo 'client_count_windows_2016 = 0' >> custom.tfvars
	echo 'consul_license = "$(shell cat $(CONSUL_LICENSE_PATH))"' >> custom.tfvars
	echo 'nomad_license = "$(shell cat $(NOMAD_LICENSE_PATH))"' >> custom.tfvars

.PHONY: plan apply clean destroy plan_full apply_full clean_full destroy_full tidy

plan: custom.tfvars
	terraform plan -var-file=custom.tfvars

apply: custom.tfvars
	terraform apply -var-file=custom.tfvars -auto-approve

destroy: custom.tfvars
	terraform destroy -var-file=custom.tfvars -auto-approve

clean: destroy tidy

# deploy what's in E2E nightly

plan_full:
	terraform plan

apply_full:
	@terraform apply -auto-approve \
		-var="consul_license=$(shell cat $(CONSUL_LICENSE_PATH))" \
		-var="nomad_license=$(shell cat $(NOMAD_LICENSE_PATH))"

clean_full: destroy_full tidy

destroy_full:
	terraform destroy -auto-approve

# util

# don't run this by default in plan/apply because it prevents you from
# updating a running cluster
tidy:
	rm -rf provision-infra/keys
	mkdir -p provision-infra/keys
	chmod 0700 provision-infra/keys
	rm -rf provision-infra/uploads/*
	git checkout uploads/README.md
	rm -f terraform.tfstate.*.backup
	rm custom.tfvars

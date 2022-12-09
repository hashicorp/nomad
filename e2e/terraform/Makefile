PKG_PATH = $(shell pwd)/../../pkg/linux_amd64/nomad
LICENSE_PATH ?=

# deploy for quick local development testing

plan:
	terraform plan \
		-var="nomad_local_binary=$(PKG_PATH)" \
		-var="volumes=false" \
		-var="client_count_ubuntu_jammy_amd64=3" \
		-var="client_count_windows_2016_amd64=0"

apply:
	terraform apply -auto-approve \
		-var="nomad_local_binary=$(PKG_PATH)" \
		-var="volumes=false" \
		-var="client_count_ubuntu_jammy_amd64=3" \
		-var="client_count_windows_2016_amd64=0"

clean: destroy tidy

destroy:
	terraform destroy -auto-approve \
		-var="nomad_local_binary=$(PKG_PATH)" \
		-var="client_count_ubuntu_jammy_amd64=3" \
		-var="client_count_windows_2016_amd64=0"

# deploy what's in E2E nightly

plan_full:
	terraform plan

apply_full:
	@terraform apply -auto-approve \
		-var="nomad_license=$(shell cat $(LICENSE_PATH))"

clean_full: destroy_full tidy

destroy_full:
	terraform destroy -auto-approve

# util

# don't run this by default in plan/apply because it prevents you from
# updating a running cluster
tidy:
	rm -rf keys
	mkdir keys
	chmod 0700 keys
	rm -rf uploads/*
	git checkout uploads/README.md
	rm -f terraform.tfstate.*.backup

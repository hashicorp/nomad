NOMAD_SHA ?= $(shell git rev-parse HEAD)
PKG_PATH = $(shell pwd)/../../pkg/linux_amd64/nomad

dev-cluster:
	terraform apply -auto-approve \
		-var="nomad_sha=$(NOMAD_SHA)"
	terraform output environment

dev-cluster-from-local:
	terraform apply -auto-approve \
		-var="nomad_local_binary=$(PKG_PATH)"
	terraform output environment

clean:
	terraform destroy -auto-approve

full-cluster:
	terraform apply -auto-approve \
		-var-file=terraform.full.tfvars \
		-var="nomad_sha=$(NOMAD_SHA)"

plan-dev-cluster:
	terraform plan \
		-var="nomad_sha=$(NOMAD_SHA)"

plan-full-cluster:
	terraform plan \
		-var-file=terraform.full.tfvars \
		-var="nomad_sha=$(NOMAD_SHA)"

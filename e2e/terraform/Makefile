NOMAD_SHA ?= $(shell git rev-parse HEAD)
PKG_PATH = $(shell pwd)/../../pkg/linux_amd64/nomad

dev-cluster:
	terraform apply -auto-approve \
		-var="nomad_sha=$(NOMAD_SHA)" \
		-var-file=terraform.tfvars.dev
	terraform output environment

dev-cluster-from-local:
	terraform apply -auto-approve \
		-var="nomad_local_binary=$(PKG_PATH)" \
		-var-file=terraform.tfvars.dev
	terraform output environment

clean:
	terraform destroy -auto-approve

full-cluster:
	terraform apply -auto-approve \
		-var="nomad_sha=$(NOMAD_SHA)" \
		-var-file=terraform.tfvars

plan-dev-cluster:
	terraform plan \
		-var="nomad_sha=$(NOMAD_SHA)" \
		-var-file=terraform.tfvars.dev

plan-full-cluster:
	terraform plan \
		-var="nomad_sha=$(NOMAD_SHA)" \
		-var-file=terraform.tfvars

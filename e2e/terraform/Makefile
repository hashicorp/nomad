NOMAD_SHA ?= $(shell git rev-parse HEAD)
PKG_PATH = $(shell pwd)/../../pkg/linux_amd64/nomad

dev-cluster:
	terraform apply -auto-approve -var-file=terraform.tfvars.dev
	terraform output provisioning | jq . > ../provisioning.json
	cd .. && NOMAD_E2E=1 go test -v . -nomad.sha=$(NOMAD_SHA) -provision.terraform ./provisioning.json -skipTests
	terraform output message

dev-cluster-from-local:
	terraform apply -auto-approve -var-file=terraform.tfvars.dev
	terraform output provisioning | jq . > ../provisioning.json
	cd .. && NOMAD_E2E=1 go test -v . -nomad.local_file=$(PKG_PATH) -provision.terraform ./provisioning.json -skipTests
	terraform output message

clean:
	terraform destroy -auto-approve

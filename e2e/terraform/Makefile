NOMAD_SHA ?= $(shell git rev-parse HEAD)

dev-cluster:
	terraform apply -auto-approve -var-file=terraform.tfvars.dev
	terraform output provisioning | jq . > ../provisioning.json
	cd .. && NOMAD_E2E=1 go test -v . -nomad.sha=$(NOMAD_SHA) -provision.terraform ./provisioning.json -skipTests
	terraform output message

clean:
	terraform destroy -auto-approve

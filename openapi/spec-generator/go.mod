module github.com/hashicorp/nomad/openapi/spec-generator

go 1.16

replace (
	github.com/hashicorp/nomad => ../..
	github.com/hashicorp/nomad/api => ../../api
	// github.com/hashicorp/nomad/testutil/openapi/v1/testclient => ../../testutil/openapi/v1/testclient
)

require (
	github.com/getkin/kin-openapi v0.69.0
	github.com/ghodss/yaml v1.0.0
	github.com/hashicorp/go-hclog v0.16.2
	github.com/hashicorp/nomad v0.0.0
	github.com/hashicorp/nomad/api v0.0.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007 // indirect
)

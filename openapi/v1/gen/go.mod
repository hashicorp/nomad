module github.com/hashicorp/nomad/openapi/v1/gen

go 1.16

replace github.com/hashicorp/nomad/api => ../../../api

require (
	github.com/hashicorp/nomad/api v0.0.0
	golang.org/x/tools v0.1.3
)

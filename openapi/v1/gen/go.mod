module github.com/hashicorp/nomad/openapi/v1/gen

go 1.16

replace github.com/hashicorp/nomad/api => ../../../api

require (
	github.com/hashicorp/nomad/api v0.0.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	golang.org/x/tools v0.1.3
)

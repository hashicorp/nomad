//+build tools

// Package tools anonymously imports packages of tools used to build nomad.
// See the GNUMakefile for 'go get` commands.
package tools

import (
	_ "github.com/a8m/tree/cmd/tree"
	_ "github.com/client9/misspell/cmd/misspell"
	_ "github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs"
	_ "github.com/golang/protobuf/protoc-gen-go"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/hashicorp/go-bindata/go-bindata"
	_ "github.com/hashicorp/go-hclog/hclogvet"
	_ "github.com/hashicorp/go-msgpack/codec/codecgen"
	_ "github.com/hashicorp/hcl/v2/cmd/hclfmt"
	_ "gotest.tools/gotestsum"
)

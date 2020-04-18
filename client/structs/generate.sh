#!/bin/bash

set -e

codecgen -d 102 -t codegen_generated -o structs.generated.go structs.go
sed -i'' -e 's|"github.com/ugorji/go/codec|"github.com/hashicorp/go-msgpack/codec|g' structs.generated.go

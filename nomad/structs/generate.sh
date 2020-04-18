#!/bin/bash
set -e

FILES="$(ls ./*.go | grep -v -e _test.go -e .generated.go | tr '\n' ' ')"
codecgen -d 100 -t codegen_generated -o structs.generated.go ${FILES}
sed -i'' -e 's|"github.com/ugorji/go/codec|"github.com/hashicorp/go-msgpack/codec|g' structs.generated.go

#!/bin/bash
set -e

# ensure that ent and pro structs don't get auto generated without tags
FILES="$(ls ./*.go | grep -v -e _test.go -e .generated.go -e _ent.go -e _pro.go -e _ent_ -e _pro_ | tr '\n' ' ')"
codecgen -d 100 -o structs.generated.go ${FILES}

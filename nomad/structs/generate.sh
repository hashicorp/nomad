#!/bin/bash
set -e

FILES="$(ls ./*.go | grep -v -e _test.go -e .generated.go | tr '\n' ' ')"
codecgen -d 100 -t codec_generated -o structs.generated.go ${FILES}

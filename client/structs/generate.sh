#!/bin/bash
set -e

FILES="$(ls *.go | tr '\n' ' ')"
codecgen -d 102 -o structs.generated.go ${FILES}

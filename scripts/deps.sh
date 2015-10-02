#!/bin/bash

# First get the OS-specific packages
GOOS=windows go get $DEP_ARGS github.com/StackExchange/wmi
GOOS=windows go get $DEP_ARGS github.com/shirou/w32

# Get the rest of the deps
DEPS=$(go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
go get $DEP_ARGS ./... $DEPS

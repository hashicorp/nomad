#!/bin/sh

cat ../nomad/structs/apistructs.go | \
    sed 's|^package structs|package api|g' \
    > nomadstructs.go

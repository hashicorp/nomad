#!/usr/bin/env bash

set -e

# Match entry in vendor.json
GIT_TAG="0053ebfd9d0ee06ccefbfe17072021e1d4acebee"
echo "Installing codec/codecgen@${GIT_TAG} ..."

# Either fetch in existing git repo or use go get to clone
git -C "$(go env GOPATH)"/src/github.com/ugorji/go/codec fetch -q || go get -d -u github.com/ugorji/go/codec/codecgen
git -C "$(go env GOPATH)"/src/github.com/ugorji/go/codec checkout --quiet $GIT_TAG
go install github.com/ugorji/go/codec/codecgen

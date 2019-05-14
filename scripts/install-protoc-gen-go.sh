#!/usr/bin/env bash

GIT_TAG="v1.2.0" # change as needed
echo "Installing protobuf/protoc-gen-go@${GIT_TAG} ..."

# Either fetch in existing repo or use go get to clone
git -C "$(go env GOPATH)"/src/github.com/golang/protobuf fetch -q || go get -d -u github.com/golang/protobuf/protoc-gen-go
git -C "$(go env GOPATH)"/src/github.com/golang/protobuf checkout --quiet $GIT_TAG
go install github.com/golang/protobuf/protoc-gen-go

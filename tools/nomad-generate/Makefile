MAKEFLAGS += --warn-undefined-variables
SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -euc
.DEFAULT_GOAL := build

GO_SRC := $(wildcard ./*.go)

.PHONY: build
build: build/nomad-generate

build/nomad-generate: $(GO_SRC)
	mkdir -p build
	go build -o build/nomad-generate .

.PHONY: install
install: build
	ln -sf $(shell pwd)/build/nomad-generate ~/bin/nomad-generate

test:
	go test -v -count=1 ./...

.PHONY: clean
clean:
	rm -rf build
	rm -f testdata/*.generated.go

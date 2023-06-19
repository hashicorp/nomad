SHELL = bash
PROJECT_ROOT := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))

GO_MODULE = github.com/hashicorp/nomad

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)
GIT_COMMIT_FLAG = $(GO_MODULE)/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)

# build date is based on most recent commit, in RFC3339 format
BUILD_DATE ?= $(shell TZ=UTC0 git show -s --format=%cd --date=format-local:'%Y-%m-%dT%H:%M:%SZ' HEAD)
BUILD_DATE_FLAG = $(GO_MODULE)/version.BuildDate=$(BUILD_DATE)

GO_LDFLAGS = -X $(GIT_COMMIT_FLAG) -X $(BUILD_DATE_FLAG)

GOPATH := $(shell go env GOPATH)

# Respect $GOBIN if set in environment or via $GOENV file.
BIN := $(shell go env GOBIN)
ifndef BIN
BIN := $(GOPATH)/bin
endif

GO_TAGS := $(GO_TAGS)

default: release

ALL_TARGETS = linux_arm

SUPPORTED_OSES = Linux

CGO_ENABLED = 1

# include per-user customization after all variables are defined
-include GNUMakefile.local

pkg/%/nomad: GO_OUT ?= $@
pkg/%/nomad: CC = arm-unknown-linux-gnueabihf-gcc
pkg/%/nomad: ## Build Nomad for GOOS_GOARCH, e.g. pkg/linux_amd64/nomad
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=$(CGO_ENABLED) \
		GOOS=$(firstword $(subst _, ,$*)) \
		GOARCH=$(lastword $(subst _, ,$*)) \
		CC=$(CC) \
		go build -trimpath -ldflags "$(GO_LDFLAGS)" -tags "$(GO_TAGS)" -o $(GO_OUT)

# Define package targets for each of the build targets we actually have on this system
define makePackageTarget

pkg/$(1).zip: pkg/$(1)/nomad
	@echo "==> Packaging for $(1)..."
	@zip -j pkg/$(1).zip pkg/$(1)/*

endef

# Reify the package targets
$(foreach t,$(ALL_TARGETS),$(eval $(call makePackageTarget,$(t))))


.PHONY: deps
deps:  ## Install build and development dependencies
	@echo "==> Updating build dependencies..."
	go install github.com/hashicorp/go-bindata/go-bindata@bf7910af899725e4938903fb32048c7c0b15f12e
	go install github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@234c15e7648ff35458026de92b34c637bae5e6f7
	go install github.com/a8m/tree/cmd/tree@fce18e2a750ea4e7f53ee706b1c3d9cbb22de79c
	go install gotest.tools/gotestsum@v1.10.0
	go install github.com/hashicorp/hcl/v2/cmd/hclfmt@d0c4fa8b0bbc2e4eeccd1ed2a32c2089ed8c5cf1
	go install github.com/golang/protobuf/protoc-gen-go@v1.3.4
	go install github.com/hashicorp/go-msgpack/codec/codecgen@v1.1.5
	go install github.com/bufbuild/buf/cmd/buf@v0.36.0
	go install github.com/hashicorp/go-changelog/cmd/changelog-build@latest
	go install golang.org/x/tools/cmd/stringer@v0.1.12
	go install github.com/hashicorp/hc-install/cmd/hc-install@v0.5.0


.PHONY: release
release: GO_TAGS=ui codegen_generated release
release: clean $(foreach t,$(ALL_TARGETS),pkg/$(t).zip) ## Build all release packages which can be built on this platform.
	@echo "==> Results:"
	@tree --dirsfirst $(PROJECT_ROOT)/pkg

.PHONY: clean
clean: GOPATH=$(shell go env GOPATH)
clean: ## Remove build artifacts
	@echo "==> Cleaning build artifacts..."
	@rm -rf "$(PROJECT_ROOT)/bin/"
	@rm -rf "$(PROJECT_ROOT)/pkg/"
	@rm -rf "$(PROJECT_ROOT)/vendor/"
	@rm -f "$(BIN)/nomad"

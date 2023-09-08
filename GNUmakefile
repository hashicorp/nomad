SHELL = bash
PROJECT_ROOT := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
THIS_OS := $(shell uname | cut -d- -f1)
THIS_ARCH := $(shell uname -m)

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

ifeq ($(CI),true)
GO_TAGS := codegen_generated $(GO_TAGS)
endif

# Don't embed the Nomad UI when the NOMAD_NO_UI env var is set.
ifndef NOMAD_NO_UI
GO_TAGS := ui $(GO_TAGS)
endif

#GOTEST_GROUP is set in CI pipelines. We have to set it for local run.
ifndef GOTEST_GROUP
GOTEST_GROUP := nomad client command drivers quick
endif

# tag corresponding to latest release we maintain backward compatibility with
PROTO_COMPARE_TAG ?= v1.0.3$(if $(findstring ent,$(GO_TAGS)),+ent,)

# LAST_RELEASE is the git sha of the latest release corresponding to this branch. main should have the latest
# published release, and release branches should point to the latest published release in the X.Y release line.
LAST_RELEASE ?= v1.6.1

default: help

ifeq (Linux,$(THIS_OS))
ALL_TARGETS = linux_386 \
	linux_amd64 \
	linux_arm \
	linux_arm64 \
	linux_s390x \
	windows_386 \
	windows_amd64
endif

ifeq (s390x,$(THIS_ARCH))
ALL_TARGETS = linux_s390x
endif

ifeq (Darwin,$(THIS_OS))
ALL_TARGETS = darwin_amd64 \
	darwin_arm64
endif

ifeq (FreeBSD,$(THIS_OS))
ALL_TARGETS = freebsd_amd64
endif

# Allow overriding ALL_TARGETS via $TARGETS
ifdef TARGETS
ALL_TARGETS = $(TARGETS)
endif

SUPPORTED_OSES = Darwin Linux FreeBSD Windows MSYS_NT

CGO_ENABLED = 1

# include per-user customization after all variables are defined
-include GNUMakefile.local

pkg/%/nomad: GO_OUT ?= $@
pkg/%/nomad: CC ?= $(shell go env CC)
pkg/%/nomad: ## Build Nomad for GOOS_GOARCH, e.g. pkg/linux_amd64/nomad
ifeq (,$(findstring $(THIS_OS),$(SUPPORTED_OSES)))
	$(warning WARNING: Building Nomad is only supported on $(SUPPORTED_OSES); not $(THIS_OS))
endif
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=$(CGO_ENABLED) \
		GOOS=$(firstword $(subst _, ,$*)) \
		GOARCH=$(lastword $(subst _, ,$*)) \
		CC=$(CC) \
		go build -trimpath -ldflags "$(GO_LDFLAGS)" -tags "$(GO_TAGS)" -o $(GO_OUT)

ifneq (armv7l,$(THIS_ARCH))
pkg/linux_arm/nomad: CC = arm-linux-gnueabihf-gcc
endif

ifneq (aarch64,$(THIS_ARCH))
pkg/linux_arm64/nomad: CC = aarch64-linux-gnu-gcc
endif

ifeq (Darwin,$(THIS_OS))
pkg/linux_%/nomad: CGO_ENABLED = 0
endif

pkg/windows_%/nomad: GO_OUT = $@.exe

# Define package targets for each of the build targets we actually have on this system
define makePackageTarget

pkg/$(1).zip: pkg/$(1)/nomad
	@echo "==> Packaging for $(1)..."
	@zip -j pkg/$(1).zip pkg/$(1)/*

endef

# Reify the package targets
$(foreach t,$(ALL_TARGETS),$(eval $(call makePackageTarget,$(t))))

.PHONY: bootstrap
bootstrap: deps lint-deps git-hooks # Install all dependencies

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
	go install github.com/shoenig/go-modtool@v0.1.1

.PHONY: lint-deps
lint-deps: ## Install linter dependencies
	@echo "==> Updating linter dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.0
	go install github.com/client9/misspell/cmd/misspell@v0.3.4
	go install github.com/hashicorp/go-hclog/hclogvet@v0.1.6

.PHONY: git-hooks
git-dir = $(shell git rev-parse --git-dir)
git-hooks: $(git-dir)/hooks/pre-push
$(git-dir)/hooks/%: dev/hooks/%
	cp $^ $@
	chmod 755 $@

.PHONY: check
check: ## Lint the source code
	@echo "==> Linting source code..."
	@golangci-lint run

	@echo "==> Linting hclog statements..."
	@hclogvet .

	@echo "==> Spell checking website..."
	@misspell -error -source=text website/pages/

	@echo "==> Checking for breaking changes in protos..."
	@buf breaking --config tools/buf/buf.yaml --against-config tools/buf/buf.yaml --against .git#tag=$(PROTO_COMPARE_TAG)

	@echo "==> Check proto files are in-sync..."
	@$(MAKE) proto
	@if (git status -s | grep -q .pb.go); then echo the following proto files are out of sync; git status -s | grep .pb.go; exit 1; fi

	@echo "==> Check format of jobspecs and HCL files..."
	@$(MAKE) hclfmt
	@if (git status -s | grep -q -e '\.hcl$$' -e '\.nomad$$' -e '\.tf$$'); then echo the following HCL files are out of sync; git status -s | grep -e '\.hcl$$' -e '\.nomad$$' -e '\.tf$$'; exit 1; fi

	@echo "==> Check API package is isolated from rest"
	@cd ./api && if go list --test -f '{{ join .Deps "\n" }}' . | grep github.com/hashicorp/nomad/ | grep -v -e /nomad/api/ -e nomad/api.test; then echo "  /api package depends the ^^ above internal nomad packages.  Remove such dependency"; exit 1; fi

	@echo "==> Check command package does not import structs"
	@cd ./command && if go list -f '{{ join .Imports "\n" }}' . | grep github.com/hashicorp/nomad/nomad/structs; then echo "  /command package imports the structs pkg. Remove such import"; exit 1; fi

	@echo "==> Checking Go mod.."
	@GO111MODULE=on $(MAKE) tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi

	@echo "==> Check raft util msg type mapping are in-sync..."
	@go generate ./helper/raftutil/
	@if (git status -s ./helper/raftutil| grep -q .go); then echo "raftutil helper message type mapping is out of sync. Run go generate ./... and push."; exit 1; fi

.PHONY: checkscripts
checkscripts: ## Lint shell scripts
	@echo "==> Linting scripts..."
	@find scripts -type f -name '*.sh' | xargs shellcheck

.PHONY: checkproto
checkproto: ## Lint protobuf files
	@echo "==> Lint proto files..."
	@buf check lint --config tools/buf/buf.yaml

	@echo "==> Checking for breaking changes in protos..."
	@buf check breaking --config tools/buf/buf.yaml --against-config tools/buf/buf.yaml --against .git#tag=$(PROTO_COMPARE_TAG)

.PHONY: generate-all
generate-all: generate-structs proto ## Generate structs, protobufs

.PHONY: generate-structs
generate-structs: LOCAL_PACKAGES = $(shell go list ./...)
generate-structs: ## Update generated code
	@echo "==> Running go generate..."
	@go generate $(LOCAL_PACKAGES)

.PHONY: proto
proto: ## Generate protobuf bindings
	@echo "==> Generating proto bindings..."
	@buf --config tools/buf/buf.yaml --template tools/buf/buf.gen.yaml generate

changelog: ## Generate changelog from entries
	@changelog-build -last-release $(LAST_RELEASE) -this-release HEAD \
		-entries-dir .changelog/ -changelog-template ./.changelog/changelog.tmpl -note-template ./.changelog/note.tmpl

## We skip the terraform directory as there are templated hcl configurations
## that do not successfully compile without rendering
.PHONY: hclfmt
hclfmt: ## Format HCL files with hclfmt
	@echo "==> Formatting HCL"
	@find . -name '.terraform' -prune \
	        -o -name 'upstart.nomad' -prune \
	        -o -name '.git' -prune \
	        -o -name 'node_modules' -prune \
	        -o -name '.next' -prune \
	        -o -path './ui/dist' -prune \
	        -o -path './website/out' -prune \
	        -o -path './command/testdata' -prune \
	        -o \( -name '*.nomad' -o -name '*.hcl' -o -name '*.tf' \) \
	      -print0 | xargs -0 hclfmt -w

.PHONY: tidy
tidy: ## Tidy up the go mod files
	@echo "==> Tidy up submodules"
	@cd tools && go mod tidy
	@cd api && go mod tidy
	@echo "==> Tidy nomad module"
	@go-modtool \
		--replace-comment="Pinned dependencies are noted in github.com/hashicorp/nomad/issues/11826." \
		--subs-comment="Nomad is built using the current source of the API module." \
		-w fmt go.mod
	@go mod tidy

.PHONY: dev
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: DEV_TARGET=pkg/$(GOOS)_$(GOARCH)/nomad
dev: hclfmt ## Build for the current development platform
	@echo "==> Removing old development build..."
	@rm -f $(PROJECT_ROOT)/$(DEV_TARGET)
	@rm -f $(PROJECT_ROOT)/bin/nomad
	@rm -f $(BIN)/nomad
	@if [ -d vendor ]; then echo -e "==> WARNING: Found vendor directory.  This may cause build errors, consider running 'rm -r vendor' or 'make clean' to remove.\n"; fi
	@$(MAKE) --no-print-directory \
		$(DEV_TARGET) \
		GO_TAGS="$(GO_TAGS) $(NOMAD_UI_TAG)"
	@mkdir -p $(PROJECT_ROOT)/bin
	@mkdir -p $(BIN)
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(PROJECT_ROOT)/bin/
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(BIN)

.PHONY: prerelease
prerelease: GO_TAGS=ui codegen_generated release
prerelease: generate-all ember-dist static-assets ## Generate all the static assets for a Nomad release

.PHONY: release
release: GO_TAGS=ui codegen_generated release
release: clean $(foreach t,$(ALL_TARGETS),pkg/$(t).zip) ## Build all release packages which can be built on this platform.
	@echo "==> Results:"
	@tree --dirsfirst $(PROJECT_ROOT)/pkg

.PHONY: test-nomad
test-nomad: GOTEST_PKGS=$(foreach g,$(GOTEST_GROUP),$(shell go run -modfile=tools/go.mod tools/missing/main.go ci/test-core.json $(g)))
test-nomad: # dev ## Run Nomad unit tests
	@echo "==> Running Nomad unit tests $(GOTEST_GROUP)"
	@echo "==> with packages $(GOTEST_PKGS)"
	gotestsum --format=testname --rerun-fails=3 --packages="$(GOTEST_PKGS)" -- \
		-cover \
		-timeout=20m \
		-count=1 \
		-tags "$(GO_TAGS)" \
		$(GOTEST_PKGS)

.PHONY: test-nomad-module
test-nomad-module: dev ## Run Nomad unit tests on sub-module
	@echo "==> Running Nomad unit tests on sub-module $(GOTEST_MOD)"
	cd $(GOTEST_MOD); gotestsum --format=testname --rerun-fails=3 --packages=./... -- \
		-cover \
		-timeout=20m \
		-count=1 \
		-race \
		-tags "$(GO_TAGS)" \
		./...

.PHONY: e2e-test
e2e-test: dev ## Run the Nomad e2e test suite
	@echo "==> Running Nomad E2E test suites:"
	go test \
		$(if $(ENABLE_RACE),-race) $(if $(VERBOSE),-v) \
		-timeout=900s \
		-tags "$(GO_TAGS)" \
		github.com/hashicorp/nomad/e2e

.PHONY: integration-test
integration-test: dev ## Run Nomad integration tests
	@echo "==> Running Nomad integration test suites:"
	NOMAD_E2E_VAULTCOMPAT=1 go test \
		-v \
		-race \
		-timeout=900s \
		-count=1 \
		-tags "$(GO_TAGS)" \
		github.com/hashicorp/nomad/e2e/vaultcompat

.PHONY: clean
clean: GOPATH=$(shell go env GOPATH)
clean: ## Remove build artifacts
	@echo "==> Cleaning build artifacts..."
	@rm -rf "$(PROJECT_ROOT)/bin/"
	@rm -rf "$(PROJECT_ROOT)/pkg/"
	@rm -rf "$(PROJECT_ROOT)/vendor/"
	@rm -f "$(BIN)/nomad"

.PHONY: testcluster
testcluster: ## Bring up a Linux test cluster using Vagrant. Set PROVIDER if necessary.
	vagrant up nomad-server01 \
		nomad-server02 \
		nomad-server03 \
		nomad-client01 \
		nomad-client02 \
		nomad-client03 \
		$(if $(PROVIDER),--provider $(PROVIDER))

.PHONY: static-assets
static-assets: ## Compile the static routes to serve alongside the API
	@echo "==> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix ui -modtime 1480000000 -tags ui -o bindata_assetfs.go ./ui/dist/...
	@mv bindata_assetfs.go command/agent

.PHONY: test-ui
test-ui: ## Run Nomad UI test suite
	@echo "==> Installing JavaScript assets"
	@cd ui && npm rebuild node-sass
	@cd ui && yarn install
	@echo "==> Running ember tests"
	@cd ui && npm test

.PHONY: ember-dist
ember-dist: ## Build the static UI assets from source
	@echo "==> Installing JavaScript assets"
	@cd ui && yarn install --silent --network-timeout 300000
	@cd ui && npm rebuild node-sass
	@echo "==> Building Ember application"
	@cd ui && npm run build

.PHONY: dev-ui
dev-ui: ember-dist static-assets ## Build a dev UI binary
	@$(MAKE) NOMAD_UI_TAG="ui" dev ## Build a dev binary with the UI baked in

HELP_FORMAT="    \033[36m%-25s\033[0m %s\n"
.PHONY: help
help: ## Display this usage information
	@echo "Valid targets:"
	@grep -E '^[^ ]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		sort | \
		awk 'BEGIN {FS = ":.*?## "}; \
			{printf $(HELP_FORMAT), $$1, $$2}'
	@echo ""
	@echo "This host will build the following targets if 'make release' is invoked:"
	@echo $(ALL_TARGETS) | sed 's/^/    /'

.PHONY: ui-screenshots
ui-screenshots: ## Collect  UI screenshots
	@echo "==> Collecting UI screenshots..."
        # Build the screenshots image if it doesn't exist yet
	@if [[ "$$(docker images -q nomad-ui-screenshots 2> /dev/null)" == "" ]]; then \
		docker build --tag="nomad-ui-screenshots" ./scripts/screenshots; \
	fi
	@docker run \
		--rm \
		--volume "$(shell pwd)/scripts/screenshots/screenshots:/screenshots" \
		nomad-ui-screenshots

.PHONY: ui-screenshots-local
ui-screenshots-local: ## Collect UI screenshots (local)
	@echo "==> Collecting UI screenshots (local)..."
	@cd scripts/screenshots/src && SCREENSHOTS_DIR="../screenshots" node index.js

.PHONY: version
version: ## Lookup the current build version
ifneq (,$(wildcard version/version_ent.go))
	@$(CURDIR)/scripts/version.sh version/version.go version/version_ent.go
else
	@$(CURDIR)/scripts/version.sh version/version.go version/version.go
endif

.PHONY: missing
missing: ## Check for packages not being tested
	@echo "==> Checking for packages not being tested ..."
	@go run -modfile tools/go.mod tools/missing/main.go ci/test-core.json

.PHONY: cl
cl: ## Create a new Changelog entry
	@go run -modfile tools/go.mod tools/cl-entry/main.go

.PHONY: test
test: GOTEST_PKGS := $(foreach g,$(GOTEST_GROUP),$(shell go run -modfile=tools/go.mod tools/missing/main.go ci/test-core.json $(g)))
test: ## Use this target as a smoke test
	@echo "==> Running Nomad smoke tests on groups: $(GOTEST_GROUP)"
	@echo "==> with packages: $(GOTEST_PKGS)"
	gotestsum --format=testname --packages="$(GOTEST_PKGS)" -- \
		-cover \
		-timeout=20m \
		-count=1 \
		-tags "$(GO_TAGS)" \
		$(GOTEST_PKGS)

SHELL = bash
PROJECT_ROOT := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
THIS_OS := $(shell uname | cut -d- -f1)
THIS_ARCH := $(shell uname -m)

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)

GO_LDFLAGS := "-X github.com/hashicorp/nomad/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"
GO_TAGS ?= codegen_generated

GO_TEST_CMD = $(if $(shell command -v gotestsum 2>/dev/null),gotestsum --,go test)

ifeq ($(origin GOTEST_PKGS_EXCLUDE), undefined)
GOTEST_PKGS ?= "./..."
else
GOTEST_PKGS=$(shell go list ./... | sed 's/github.com\/hashicorp\/nomad/./' | egrep -v "^($(GOTEST_PKGS_EXCLUDE))(/.*)?$$")
endif

# tag corresponding to latest release we maintain backward compatibility with
PROTO_COMPARE_TAG ?= v1.0.0$(if $(findstring ent,$(GO_TAGS)),+ent,)

default: help

ifeq ($(CI),true)
	$(info Running in a CI environment, verbose mode is disabled)
else
	VERBOSE="true"
endif

ifeq (Linux,$(THIS_OS))
ALL_TARGETS = linux_386 \
	linux_amd64 \
	linux_arm \
	linux_arm64 \
	windows_386 \
	windows_amd64
endif

ifeq (s390x,$(THIS_ARCH))
ALL_TARGETS = linux_s390x
endif

ifeq (Darwin,$(THIS_OS))
ALL_TARGETS = darwin_amd64
endif

ifeq (FreeBSD,$(THIS_OS))
ALL_TARGETS = freebsd_amd64
endif

SUPPORTED_OSES = Darwin Linux FreeBSD Windows MSYS_NT

# include per-user customization after all variables are defined
-include GNUMakefile.local

pkg/%/nomad: GO_OUT ?= $@
pkg/%/nomad: CC ?= $(shell go env CC)
pkg/%/nomad: ## Build Nomad for GOOS_GOARCH, e.g. pkg/linux_amd64/nomad
ifeq (,$(findstring $(THIS_OS),$(SUPPORTED_OSES)))
	$(warning WARNING: Building Nomad is only supported on $(SUPPORTED_OSES); not $(THIS_OS))
endif
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 \
		GOOS=$(firstword $(subst _, ,$*)) \
		GOARCH=$(lastword $(subst _, ,$*)) \
		CC=$(CC) \
		go build -trimpath -ldflags $(GO_LDFLAGS) -tags "$(GO_TAGS)" -o $(GO_OUT)

pkg/linux_arm/nomad: CC = arm-linux-gnueabihf-gcc-5
pkg/linux_arm64/nomad: CC = aarch64-linux-gnu-gcc-5
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
## Keep versions in sync with tools/go.mod for now (see https://github.com/golang/go/issues/30515)
	@echo "==> Updating build dependencies..."
	GO111MODULE=on cd tools && go get github.com/hashicorp/go-bindata/go-bindata@bf7910af899725e4938903fb32048c7c0b15f12e
	GO111MODULE=on cd tools && go get github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@234c15e7648ff35458026de92b34c637bae5e6f7
	GO111MODULE=on cd tools && go get github.com/a8m/tree/cmd/tree@fce18e2a750ea4e7f53ee706b1c3d9cbb22de79c
	GO111MODULE=on cd tools && go get gotest.tools/gotestsum@v0.4.2
	GO111MODULE=on cd tools && go get github.com/hashicorp/hcl/v2/cmd/hclfmt@v2.5.1
	GO111MODULE=on cd tools && go get github.com/golang/protobuf/protoc-gen-go@v1.3.4
	GO111MODULE=on cd tools && go get github.com/hashicorp/go-msgpack/codec/codecgen@v1.1.5

.PHONY: lint-deps
lint-deps: ## Install linter dependencies
## Keep versions in sync with tools/go.mod (see https://github.com/golang/go/issues/30515)
	@echo "==> Updating linter dependencies..."
	GO111MODULE=on cd tools && go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.24.0
	GO111MODULE=on cd tools && go get github.com/client9/misspell/cmd/misspell@v0.3.4
	GO111MODULE=on cd tools && go get github.com/hashicorp/go-hclog/hclogvet@v0.1.3

.PHONY: git-hooks
git-dir = $(shell git rev-parse --git-dir)
git-hooks: $(git-dir)/hooks/pre-push
$(git-dir)/hooks/%: dev/hooks/%
	cp $^ $@
	chmod 755 $@

.PHONY: check
check: ## Lint the source code
	@echo "==> Linting source code..."
	@golangci-lint run -j 1

	@echo "==> Linting hclog statements..."
	@hclogvet .

	@echo "==> Spell checking website..."
	@misspell -error -source=text website/pages/

	@echo "==> Checking for breaking changes in protos..."
	@buf check breaking --config tools/buf/buf.yaml --against-config tools/buf/buf.yaml --against .git#tag=$(PROTO_COMPARE_TAG)

	@echo "==> Check proto files are in-sync..."
	@$(MAKE) proto
	@if (git status -s | grep -q .pb.go); then echo the following proto files are out of sync; git status -s | grep .pb.go; exit 1; fi

	@echo "==> Check format of jobspecs and HCL files..."
	@$(MAKE) hclfmt
	@if (git status -s | grep -q -e '\.hcl$$' -e '\.nomad$$'); then echo the following HCL files are out of sync; git status -s | grep -e '\.hcl$$' -e '\.nomad$$'; exit 1; fi

	@echo "==> Check API package is isolated from rest"
	@cd ./api && if go list --test -f '{{ join .Deps "\n" }}' . | grep github.com/hashicorp/nomad/ | grep -v -e /vendor/ -e /nomad/api/ -e nomad/api.test; then echo "  /api package depends the ^^ above internal nomad packages.  Remove such dependency"; exit 1; fi

	@echo "==> Checking Go mod.."
	@GO111MODULE=on $(MAKE) sync
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
	@if (git status --porcelain | grep -Eq "vendor/github.com/hashicorp/nomad/.*\.go"); then \
		echo "nomad go submodules are out of sync, try 'make sync':"; \
		git status -s | grep -E "vendor/github.com/hashicorp/nomad/.*\.go"; \
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
generate-all: generate-structs proto generate-examples

.PHONY: generate-structs
generate-structs: LOCAL_PACKAGES = $(shell go list ./... | grep -v '/vendor/')
generate-structs: ## Update generated code
	@echo "--> Running go generate..."
	@go generate $(LOCAL_PACKAGES)

.PHONY: proto
proto:
	@echo "--> Generating proto bindings..."
	@buf --config tools/buf/buf.yaml --template tools/buf/buf.gen.yaml generate

.PHONY: generate-examples
generate-examples: command/job_init.bindata_assetfs.go

command/job_init.bindata_assetfs.go: command/assets/*
	go-bindata-assetfs -pkg command -o command/job_init.bindata_assetfs.go ./command/assets/...

.PHONY: changelogfmt
changelogfmt:
	@echo "--> Making [GH-xxxx] references clickable..."
	@sed -E 's|([^\[])\[GH-([0-9]+)\]|\1[[GH-\2](https://github.com/hashicorp/nomad/issues/\2)]|g' CHANGELOG.md > changelog.tmp && mv changelog.tmp CHANGELOG.md

## We skip the terraform directory as there are templated hcl configurations
## that do not successfully compile without rendering
.PHONY: hclfmt
hclfmt:
	@echo "--> Formatting HCL"
	@find . -path ./terraform -prune -o -name 'upstart.nomad' -prune -o \( -name '*.nomad' -o -name '*.hcl' \) -exec \
sh -c 'hclfmt -w {} || echo in path {}' ';'

.PHONY: tidy
tidy:
	@echo "--> Tidy up submodules"
	@cd tools && go mod tidy
	@cd api && go mod tidy
	@echo "--> Tidy nomad module"
	@go mod tidy

.PHONY: sync
sync: tidy
	@echo "--> Sync vendor directory"
	@go mod vendor

.PHONY: dev
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: GOPATH=$(shell go env GOPATH)
dev: DEV_TARGET=pkg/$(GOOS)_$(GOARCH)/nomad
dev: changelogfmt hclfmt ## Build for the current development platform
	@echo "==> Removing old development build..."
	@rm -f $(PROJECT_ROOT)/$(DEV_TARGET)
	@rm -f $(PROJECT_ROOT)/bin/nomad
	@rm -f $(GOPATH)/bin/nomad
	@$(MAKE) --no-print-directory \
		$(DEV_TARGET) \
		GO_TAGS="$(GO_TAGS) $(NOMAD_UI_TAG)"
	@mkdir -p $(PROJECT_ROOT)/bin
	@mkdir -p $(GOPATH)/bin
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(PROJECT_ROOT)/bin/
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(GOPATH)/bin

.PHONY: prerelease
prerelease: GO_TAGS=ui codegen_generated release
prerelease: generate-all ember-dist static-assets ## Generate all the static assets for a Nomad release

.PHONY: release
release: GO_TAGS=ui codegen_generated release
release: clean $(foreach t,$(ALL_TARGETS),pkg/$(t).zip) ## Build all release packages which can be built on this platform.
	@echo "==> Results:"
	@tree --dirsfirst $(PROJECT_ROOT)/pkg

.PHONY: test
test: ## Run the Nomad test suite and/or the Nomad UI test suite
	@if [ ! $(SKIP_NOMAD_TESTS) ]; then \
		make test-nomad; \
		fi
	@if [ $(RUN_WEBSITE_TESTS) ]; then \
		make test-website; \
		fi
	@if [ $(RUN_UI_TESTS) ]; then \
		make test-ui; \
		fi
	@if [ $(RUN_E2E_TESTS) ]; then \
		make e2e-test; \
		fi

.PHONY: test-nomad
test-nomad: dev ## Run Nomad test suites
	@echo "==> Running Nomad test suites:"
	$(if $(ENABLE_RACE),GORACE="strip_path_prefix=$(GOPATH)/src") $(GO_TEST_CMD) \
		$(if $(ENABLE_RACE),-race) $(if $(VERBOSE),-v) \
		-cover \
		-timeout=15m \
		-tags "$(GO_TAGS)" \
		$(GOTEST_PKGS) $(if $(VERBOSE), >test.log ; echo $$? > exit-code)
	@if [ $(VERBOSE) ] ; then \
		bash -C "$(PROJECT_ROOT)/scripts/test_check.sh" ; \
	fi

.PHONY: test-nomad-module
test-nomad-module: dev ## Run Nomad test suites on a sub-module
	@echo "==> Running Nomad test suites on sub-module:"
	@cd $(GOTEST_MOD) && $(if $(ENABLE_RACE),GORACE="strip_path_prefix=$(GOPATH)/src") $(GO_TEST_CMD) \
		$(if $(ENABLE_RACE),-race) $(if $(VERBOSE),-v) \
		-cover \
		-timeout=15m \
		-tags "$(GO_TAGS)" \
		./... $(if $(VERBOSE), >test.log ; echo $$? > exit-code)
	@if [ $(VERBOSE) ] ; then \
		bash -C "$(PROJECT_ROOT)/scripts/test_check.sh" ; \
	fi

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
	go test \
		$(if $(ENABLE_RACE),-race) $(if $(VERBOSE),-v) \
		-cover \
		-timeout=900s \
		-tags "$(GO_TAGS)" \
		github.com/hashicorp/nomad/e2e/vaultcompat/ \
		-integration

.PHONY: clean
clean: GOPATH=$(shell go env GOPATH)
clean: ## Remove build artifacts
	@echo "==> Cleaning build artifacts..."
	@rm -rf "$(PROJECT_ROOT)/bin/"
	@rm -rf "$(PROJECT_ROOT)/pkg/"
	@rm -f "$(GOPATH)/bin/nomad"

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
	@echo "--> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix ui -modtime 1480000000 -tags ui -o bindata_assetfs.go ./ui/dist/...
	@mv bindata_assetfs.go command/agent

.PHONY: test-ui
test-ui: ## Run Nomad UI test suite
	@echo "--> Installing JavaScript assets"
	@cd ui && npm rebuild node-sass
	@cd ui && yarn install
	@echo "--> Running ember tests"
	@cd ui && npm test

.PHONY: ember-dist
ember-dist: ## Build the static UI assets from source
	@echo "--> Installing JavaScript assets"
	@cd ui && yarn install --silent
	@cd ui && npm rebuild node-sass
	@echo "--> Building Ember application"
	@cd ui && npm run build

.PHONY: dev-ui
dev-ui: ember-dist static-assets
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
ui-screenshots:
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
ui-screenshots-local:
	@echo "==> Collecting UI screenshots (local)..."
	@cd scripts/screenshots/src && SCREENSHOTS_DIR="../screenshots" node index.js

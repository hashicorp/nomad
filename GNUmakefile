SHELL = bash
PROJECT_ROOT := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
THIS_OS := $(shell uname)

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)

GO_LDFLAGS := "-X github.com/hashicorp/nomad/version.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"
GO_TAGS =

default: help

ifeq (,$(findstring $(THIS_OS),Darwin Linux FreeBSD))
$(error Building Nomad is currently only supported on Darwin and Linux.)
endif

# On Linux we build for Linux, Windows, and potentially Linux+LXC
ifeq (Linux,$(THIS_OS))

# Detect if we have LXC on the path
ifeq (0,$(shell pkg-config --exists lxc; echo $$?))
HAS_LXC="true"
endif

ifeq ($(TRAVIS),true)
$(info Running in Travis, verbose mode is disabled)
else
VERBOSE="true"
endif


ALL_TARGETS += linux_386 \
	linux_amd64 \
	linux_arm \
	linux_arm64 \
	windows_386 \
	windows_amd64

ifeq ("true",$(HAS_LXC))
ALL_TARGETS += linux_amd64-lxc
endif
endif

# On MacOS, we only build for MacOS
ifeq (Darwin,$(THIS_OS))
ALL_TARGETS += darwin_amd64
endif

# On FreeBSD, we only build for FreeBSD
ifeq (FreeBSD,$(THIS_OS))
ALL_TARGETS += freebsd_amd64
endif

pkg/darwin_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for darwin/amd64
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/freebsd_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for freebsd/amd64
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=freebsd GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_386/nomad: $(SOURCE_FILES) ## Build Nomad for linux/386
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=386 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for linux/amd64
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_arm/nomad: $(SOURCE_FILES) ## Build Nomad for linux/arm
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=arm CC=arm-linux-gnueabihf-gcc-5 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_arm64/nomad: $(SOURCE_FILES) ## Build Nomad for linux/arm64
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc-5 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

# If CGO support for Windows is ever required, set the following variables
# in the environment for `go build` for both the windows/amd64 and the
# windows/386 targets:
#	CC=i686-w64-mingw32-gcc
#	CXX=i686-w64-mingw32-g++
pkg/windows_386/nomad: $(SOURCE_FILES) ## Build Nomad for windows/386
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=windows GOARCH=386 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@.exe"

pkg/windows_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for windows/amd64
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@.exe"

pkg/linux_amd64-lxc/nomad: $(SOURCE_FILES) ## Build Nomad+LXC for linux/amd64
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS) lxc" \
		-o "$@"

# Define package targets for each of the build targets we actually have on this system
define makePackageTarget

pkg/$(1).zip: pkg/$(1)/nomad
	@echo "==> Packaging for $(1)..."
	@zip -j pkg/$(1).zip pkg/$(1)/*

endef

# Reify the package targets
$(foreach t,$(ALL_TARGETS),$(eval $(call makePackageTarget,$(t))))

.PHONY: bootstrap
bootstrap: deps lint-deps # Install all dependencies

.PHONY: deps
deps:  ## Install build and development dependencies
	@echo "==> Updating build dependencies..."
	go get -u github.com/kardianos/govendor
	go get -u github.com/ugorji/go/codec/codecgen
	go get -u github.com/hashicorp/go-bindata/...
	go get -u github.com/elazarl/go-bindata-assetfs/...
	go get -u github.com/a8m/tree/cmd/tree
	go get -u github.com/magiconair/vendorfmt/cmd/vendorfmt
	go get -u github.com/golang/protobuf/protoc-gen-go

.PHONY: lint-deps
lint-deps: ## Install linter dependencies
	@echo "==> Updating linter dependencies..."
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install

.PHONY: check
check: ## Lint the source code
	@echo "==> Linting source code..."
	@gometalinter \
		--deadline 10m \
		--vendor \
		--exclude='.*\.generated\.go' \
		--exclude='.*bindata_assetfs\.go' \
		--skip="ui/" \
		--sort="path" \
		--aggregate \
		--enable-gc \
		--disable-all \
		--enable goimports \
		--enable misspell \
		--enable vet \
		--enable deadcode \
		--enable varcheck \
		--enable ineffassign \
		--enable structcheck \
		--enable unconvert \
		--enable gofmt \
		./...
	@echo "==> Spell checking website..."
	@misspell -error -source=text website/source/

.PHONY: checkscripts
checkscripts: ## Lint shell scripts
	@echo "==> Linting scripts..."
	@shellcheck ./scripts/*

generate: LOCAL_PACKAGES = $(shell go list ./... | grep -v '/vendor/')
generate: ## Update generated code
	@go generate $(LOCAL_PACKAGES)

vendorfmt:
	@echo "--> Formatting vendor/vendor.json"
	test -x $(GOPATH)/bin/vendorfmt || go get -u github.com/magiconair/vendorfmt/cmd/vendorfmt
		vendorfmt
changelogfmt:
	@echo "--> Making [GH-xxxx] references clickable..."
	@sed -E 's|([^\[])\[GH-([0-9]+)\]|\1[[GH-\2](https://github.com/hashicorp/nomad/issues/\2)]|g' CHANGELOG.md > changelog.tmp && mv changelog.tmp CHANGELOG.md


.PHONY: dev
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: GOPATH=$(shell go env GOPATH)
dev: DEV_TARGET=pkg/$(GOOS)_$(GOARCH)$(if $(HAS_LXC),-lxc)/nomad
dev: vendorfmt changelogfmt ## Build for the current development platform
	@echo "==> Removing old development build..."
	@rm -f $(PROJECT_ROOT)/$(DEV_TARGET)
	@rm -f $(PROJECT_ROOT)/bin/nomad
	@rm -f $(GOPATH)/bin/nomad
	@$(MAKE) --no-print-directory \
		$(DEV_TARGET) \
		GO_TAGS="$(NOMAD_UI_TAG)"
	@mkdir -p $(PROJECT_ROOT)/bin
	@mkdir -p $(GOPATH)/bin
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(PROJECT_ROOT)/bin/
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(GOPATH)/bin

.PHONY: prerelease
prerelease: GO_TAGS=ui release
prerelease: check generate ember-dist static-assets ## Generate all the static assets for a Nomad release

.PHONY: release
release: GO_TAGS=ui release
release: clean $(foreach t,$(ALL_TARGETS),pkg/$(t).zip) ## Build all release packages which can be built on this platform.
	@echo "==> Results:"
	@tree --dirsfirst $(PROJECT_ROOT)/pkg

.PHONY: test
test: ## Run the Nomad test suite and/or the Nomad UI test suite
	@if [ ! $(SKIP_NOMAD_TESTS) ]; then \
		make test-nomad; \
		fi
	@if [ $(RUN_UI_TESTS) ]; then \
		make test-ui; \
		fi

.PHONY: test-nomad
test-nomad: dev ## Run Nomad test suites
	@echo "==> Running Nomad test suites:"
	$(if $(ENABLE_RACE),GORACE="strip_path_prefix=$(GOPATH)/src") go test \
		$(if $(ENABLE_RACE),-race) $(if $(VERBOSE),-v) \
		-cover \
		-timeout=900s \
		-tags="$(if $(HAS_LXC),lxc)" ./... $(if $(VERBOSE), >test.log ; echo $$? > exit-code)
	@if [ $(VERBOSE) ] ; then \
		bash -C "$(PROJECT_ROOT)/scripts/test_check.sh" ; \
	fi

.PHONY: clean
clean: GOPATH=$(shell go env GOPATH)
clean: ## Remove build artifacts
	@echo "==> Cleaning build artifacts..."
	@rm -rf "$(PROJECT_ROOT)/bin/"
	@rm -rf "$(PROJECT_ROOT)/pkg/"
	@rm -f "$(GOPATH)/bin/nomad"

.PHONY: travis
travis: ## Run Nomad test suites with output to prevent timeouts under Travis CI
	@if [ ! $(SKIP_NOMAD_TESTS) ]; then \
		make generate; \
	fi
	@sh -C "$(PROJECT_ROOT)/scripts/travis.sh"

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

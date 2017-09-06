PROJECT_ROOT := $(patsubst %/,%,$(dir $(abspath $(lastword $(MAKEFILE_LIST)))))
THIS_OS := $(shell uname)

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_DIRTY := $(if $(shell git status --porcelain),+CHANGES)

GO_LDFLAGS := "-X main.GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"
GO_TAGS =

# Enable additional linters as the codebase evolves to pass them
CHECKS ?= --enable goimports

default: help

ifeq (,$(findstring $(THIS_OS),Darwin Linux))
$(error Building Nomad is currently only supported on Darwin and Linux.)
endif

# On Linux we build for Linux, Windows, and potentially Linux+LXC
ifeq (Linux,$(THIS_OS))

# Detect if we have LXC on the path
ifeq (0,$(shell pkg-config --exists lxc; echo $$?))
HAS_LXC="true"
endif

ALL_TARGETS += linux_386 \
	linux_amd64 \
	linux_arm \
	linux_arm64 \
	windows_386 \
	windows_amd64

ifeq (,$(HAS_LXC))
ALL_TARGETS += linux_amd64-lxc
endif
endif

# On MacOS we only build for MacOS
ifeq (Darwin,$(THIS_OS))
ALL_TARGETS += darwin_amd64
endif

pkg/darwin_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for darwin/amd64
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_386/nomad: $(SOURCE_FILES) ## Build Nomad for linux/386
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=386 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for linux/amd64
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_arm/nomad: $(SOURCE_FILES) ## Build Nomad for linux/arm
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=arm CC=arm-linux-gnueabihf-gcc-5 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@"

pkg/linux_arm64/nomad: $(SOURCE_FILES) ## Build Nomad for linux/arm64
	@echo "==> Building $@..."
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
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=windows GOARCH=386 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@.exe"

pkg/windows_amd64/nomad: $(SOURCE_FILES) ## Build Nomad for windows/amd64
	@echo "==> Building $@..."
	@CGO_ENABLED=1 GOOS=windows GOARCH=amd64 \
		go build \
		-ldflags $(GO_LDFLAGS) \
		-tags "$(GO_TAGS)" \
		-o "$@.exe"

pkg/linux_amd64-lxc/nomad: $(SOURCE_FILES) ## Build Nomad+LXC for linux/amd64
	@echo "==> Building $@..."
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

# Only for Travis CI compliance
.PHONY: bootstrap
bootstrap: deps

.PHONY: deps
deps: ## Install build and development dependencies
	@echo "==> Updating build dependencies..."
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
	go get -u github.com/kardianos/govendor
	go get -u golang.org/x/tools/cmd/cover
	go get -u github.com/axw/gocov/gocov
	go get -u gopkg.in/matm/v1/gocov-html
	go get -u github.com/ugorji/go/codec/codecgen
	go get -u github.com/hashicorp/vault
	go get -u github.com/a8m/tree/cmd/tree

.PHONY: check
check: ## Lint the source code
	@echo "==> Linting source code..."
	@gometalinter \
		--deadline 10m \
		--vendor \
		--exclude '.*\.generated\.go:\d+:' \
		--disable-all \
		--sort severity \
		$(CHECKS) \
		./...

generate: LOCAL_PACKAGES = $(shell go list ./... | grep -v '/vendor/')
generate: ## Update generated code
	@go generate $(LOCAL_PACKAGES)

.PHONY: dev
dev: GOOS=$(shell go env GOOS)
dev: GOARCH=$(shell go env GOARCH)
dev: GOPATH=$(shell go env GOPATH)
dev: DEV_TARGET=pkg/$(GOOS)_$(GOARCH)$(if $(HAS_LXC),-lxc)/nomad
dev: check ## Build for the current development platform
	@echo "==> Removing old development build..."
	@rm -f $(PROJECT_ROOT)/$(DEV_TARGET)
	@rm -f $(PROJECT_ROOT)/bin/nomad
	@rm -f $(GOPATH)/bin/nomad
	@$(MAKE) --no-print-directory \
		$(DEV_TARGET) \
		GO_TAGS=nomad_test
	@mkdir -p $(PROJECT_ROOT)/bin
	@mkdir -p $(GOPATH)/bin
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(PROJECT_ROOT)/bin/
	@cp $(PROJECT_ROOT)/$(DEV_TARGET) $(GOPATH)/bin

.PHONY: release
release: clean check $(foreach t,$(ALL_TARGETS),pkg/$(t).zip) ## Build all release packages which can be built on this platform.
	@echo "==> Results:"
	@tree --dirsfirst $(PROJECT_ROOT)/pkg

.PHONY: test
test: LOCAL_PACKAGES = $(shell go list ./... | grep -v '/vendor/')
test: dev ## Run Nomad test suites
	@echo "==> Running Nomad test suites:"
	@NOMAD_TEST_RKT=1 \
		go test \
			-cover \
			-timeout=900s \
			-tags="nomad_test $(if $(HAS_LXC),lxc)" \
			$(LOCAL_PACKAGES)

.PHONY: clean
clean: GOPATH=$(shell go env GOPATH)
clean: ## Remove build artifacts
	@echo "==> Cleaning build artifacts..."
	@rm -rf "$(PROJECT_ROOT)/bin/"
	@rm -rf "$(PROJECT_ROOT)/pkg/"
	@rm -f "$(GOPATH)/bin/nomad"

.PHONY: travis
travis: ## Run Nomad test suites with output to prevent timeouts under Travis CI
	@sh -C "$(PROJECT_ROOT)/scripts/travis.sh"

HELP_FORMAT="    \033[36m%-25s\033[0m %s\n"
.PHONY: help
help: ## Display this usage information
	@echo "Valid targets:"
	@grep -E '^[^ ]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		sort | \
		awk 'BEGIN {FS = ":.*?## "}; \
			{printf $(HELP_FORMAT), $$1, $$2}'
	@echo "\nThis host will build the following targets if 'make release' is invoked:"
	@echo $(ALL_TARGETS) | sed 's/^/    /'

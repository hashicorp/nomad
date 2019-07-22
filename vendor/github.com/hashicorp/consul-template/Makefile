# Metadata about this makefile and position
MKFILE_PATH := $(lastword $(MAKEFILE_LIST))
CURRENT_DIR := $(patsubst %/,%,$(dir $(realpath $(MKFILE_PATH))))

# Ensure GOPATH
GOPATH ?= $(shell go env GOPATH)
# assume last entry in GOPATH is home to project
GOPATH := $(lastword $(subst :, ,${GOPATH}))

# Tags specific for building
GOTAGS ?=

# Number of procs to use
GOMAXPROCS ?= 4

# Get the project metadata
GOVERSION := 1.12.5
PROJECT := $(CURRENT_DIR:$(GOPATH)/src/%=%)
OWNER := $(notdir $(patsubst %/,%,$(dir $(PROJECT))))
NAME := $(notdir $(PROJECT))
GIT_COMMIT ?= $(shell git rev-parse --short HEAD)
VERSION := $(shell awk -F\" '/Version/ { print $$2; exit }' "${CURRENT_DIR}/version/version.go")
EXTERNAL_TOOLS = \
	github.com/golang/dep/cmd/dep

# Current system information
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Default os-arch combination to build
XC_OS ?= darwin freebsd linux netbsd openbsd solaris windows
XC_ARCH ?= 386 amd64 arm
XC_EXCLUDE ?= darwin/arm solaris/386 solaris/arm windows/arm

# GPG Signing key (blank by default, means no GPG signing)
GPG_KEY ?=

# List of ldflags
LD_FLAGS ?= \
	-s \
	-w \
	-X ${PROJECT}/version.Name=${NAME} \
	-X ${PROJECT}/version.GitCommit=${GIT_COMMIT}

# List of Docker targets to build
DOCKER_TARGETS ?= alpine light scratch

# List of tests to run
TEST ?= ./...

# Create a cross-compile target for every os-arch pairing. This will generate
# a make target for each os/arch like "make linux/amd64" as well as generate a
# meta target (build) for compiling everything.
define make-xc-target
  $1/$2:
  ifneq (,$(findstring ${1}/${2},$(XC_EXCLUDE)))
		@printf "%s%20s %s\n" "-->" "${1}/${2}:" "${PROJECT} (excluded)"
  else
		@printf "%s%20s %s\n" "-->" "${1}/${2}:" "${PROJECT}"
		env \
			CGO_ENABLED="0" \
			GOOS="${1}" \
			GOARCH="${2}" \
			go build \
				-a \
				-o="pkg/${1}_${2}/${NAME}${3}" \
				-ldflags "${LD_FLAGS}" \
				-tags "${GOTAGS}"
  endif
  .PHONY: $1/$2

  $1:: $1/$2
  .PHONY: $1

  build:: $1/$2
  .PHONY: build
endef
$(foreach goarch,$(XC_ARCH),$(foreach goos,$(XC_OS),$(eval $(call make-xc-target,$(goos),$(goarch),$(if $(findstring windows,$(goos)),.exe,)))))

# Use docker to create pristine builds for release
pristine:
	@docker run \
		--interactive \
		--user $$(id -u):$$(id -g) \
		--rm \
		--dns="8.8.8.8" \
		--volume="${CURRENT_DIR}:/go/src/${PROJECT}" \
		--workdir="/go/src/${PROJECT}" \
		"golang:${GOVERSION}" env GOCACHE=/tmp make -j4 build

# bootstrap installs the necessary go tools for development or build.
bootstrap:
	@echo "==> Bootstrapping ${PROJECT}"
	@for t in ${EXTERNAL_TOOLS}; do \
		echo "--> Installing $$t" ; \
		go get -u "$$t"; \
	done
.PHONY: bootstrap

# deps updates all dependencies for this project.
deps:
	@echo "==> Updating deps for ${PROJECT}"
	@dep ensure -update
	@dep prune
.PHONY: deps

# dev builds and installs the project locally.
dev:
	@echo "==> Installing ${NAME} for ${GOOS}/${GOARCH}"
	@rm -f "${GOPATH}/pkg/${GOOS}_${GOARCH}/${PROJECT}/version.a" # ldflags change and go doesn't detect
	@env \
		CGO_ENABLED="0" \
		go install \
			-ldflags "${LD_FLAGS}" \
			-tags "${GOTAGS}"
.PHONY: dev

# dist builds the binaries and then signs and packages them for distribution
dist:
	@$(MAKE) -f "${MKFILE_PATH}" _cleanup
	@$(MAKE) -f "${MKFILE_PATH}" pristine
	@$(MAKE) -f "${MKFILE_PATH}" _compress _checksum
.PHONY: dist

release: dist
ifndef GPG_KEY
	@echo "==> ERROR: No GPG key specified! Without a GPG key, this release cannot"
	@echo "           be signed. Set the environment variable GPG_KEY to the ID of"
	@echo "           the GPG key to continue."
	@exit 127
else
	@$(MAKE) -f "${MKFILE_PATH}" _sign
endif
.PHONY: release

# Create a docker compile and push target for each container. This will create
# docker-build/scratch, docker-push/scratch, etc. It will also create two meta
# targets: docker-build and docker-push, which will build and push all
# configured Docker containers. Each container must have a folder in docker/
# named after itself with a Dockerfile (docker/alpine/Dockerfile).
define make-docker-target
  docker-build/$1:
		@echo "==> Building ${1} Docker container for ${PROJECT}"
		@docker build \
			--rm \
			--force-rm \
			--no-cache \
			--compress \
			--file="docker/${1}/Dockerfile" \
			--build-arg="LD_FLAGS=${LD_FLAGS}" \
			--build-arg="GOTAGS=${GOTAGS}" \
			$(if $(filter $1,scratch),--tag="${OWNER}/${NAME}",) \
			--tag="${OWNER}/${NAME}:${1}" \
			--tag="${OWNER}/${NAME}:${VERSION}-${1}" \
			"${CURRENT_DIR}"
  .PHONY: docker-build/$1

  docker-build:: docker-build/$1
  .PHONY: docker-build

  docker-push/$1:
		@echo "==> Pushing ${1} to Docker registry"
		$(if $(filter $1,scratch),@docker push "${OWNER}/${NAME}",)
		@docker push "${OWNER}/${NAME}:${1}"
		@docker push "${OWNER}/${NAME}:${VERSION}-${1}"
  .PHONY: docker-push/$1

  docker-push:: docker-push/$1
  .PHONY: docker-push
endef
$(foreach target,$(DOCKER_TARGETS),$(eval $(call make-docker-target,$(target))))

# test runs the test suite.
test:
	@echo "==> Testing ${NAME}"
	@go test -timeout=30s -parallel=20 -failfast -tags="${GOTAGS}" ./... ${TESTARGS}
.PHONY: test

# test-race runs the test suite.
test-race:
	@echo "==> Testing ${NAME} (race)"
	@go test -timeout=60s -race -tags="${GOTAGS}" ./... ${TESTARGS}
.PHONY: test-race

# _cleanup removes any previous binaries
_cleanup:
	@rm -rf "${CURRENT_DIR}/pkg/"
	@rm -rf "${CURRENT_DIR}/bin/"
.PHONY: _cleanup

clean: _cleanup
.PHONY: clean

# _compress compresses all the binaries in pkg/* as tarball and zip.
_compress:
	@mkdir -p "${CURRENT_DIR}/pkg/dist"
	@for platform in $$(find ./pkg -mindepth 1 -maxdepth 1 -type d); do \
		osarch=$$(basename "$$platform"); \
		if [ "$$osarch" = "dist" ]; then \
			continue; \
		fi; \
		\
		ext=""; \
		if test -z "$${osarch##*windows*}"; then \
			ext=".exe"; \
		fi; \
		cd "$$platform"; \
		tar -czf "${CURRENT_DIR}/pkg/dist/${NAME}_${VERSION}_$${osarch}.tgz" "${NAME}$${ext}"; \
		zip -q "${CURRENT_DIR}/pkg/dist/${NAME}_${VERSION}_$${osarch}.zip" "${NAME}$${ext}"; \
		cd - >/dev/null; \
	done
.PHONY: _compress

# _checksum produces the checksums for the binaries in pkg/dist
_checksum:
	@cd "${CURRENT_DIR}/pkg/dist" && \
		shasum --algorithm 256 * > ${CURRENT_DIR}/pkg/dist/${NAME}_${VERSION}_SHA256SUMS && \
		cd - >/dev/null
.PHONY: _checksum

# _sign signs the binaries using the given GPG_KEY. This should not be called
# as a separate function.
_sign:
	@echo "==> Signing ${PROJECT} at v${VERSION}"
	@gpg \
		--default-key "${GPG_KEY}" \
		--detach-sig "${CURRENT_DIR}/pkg/dist/${NAME}_${VERSION}_SHA256SUMS"
	@git commit \
		--allow-empty \
		--gpg-sign="${GPG_KEY}" \
		--message "Release v${VERSION}" \
		--quiet \
		--signoff
	@git tag \
		--annotate \
		--create-reflog \
		--local-user "${GPG_KEY}" \
		--message "Version ${VERSION}" \
		--sign \
		"v${VERSION}" master
	@echo "--> Do not forget to run:"
	@echo ""
	@echo "    git push && git push --tags"
	@echo ""
	@echo "And then upload the binaries in dist/!"
.PHONY: _sign

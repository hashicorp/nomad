PACKAGES = $(shell go list ./...)
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
EXTERNAL_TOOLS=\
	github.com/tools/godep \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/vet \
	github.com/axw/gocov/gocov \
	gopkg.in/matm/v1/gocov-html

all: test

dev: deps format
	@NOMAD_DEV=1 sh -c "'$(CURDIR)/scripts/build.sh'"

bin:
	@sh -c "'$(CURDIR)/scripts/build.sh'"

release: updatedeps
	@$(MAKE) bin

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

deps:
	@echo "--> Installing build dependencies"
	@DEP_ARGS="-d -v" sh -c "'$(CURDIR)/scripts/deps.sh'"

updatedeps: deps
	@echo "--> Updating build dependencies"
	@DEP_ARGS="-d -f -u" sh -c "'$(CURDIR)/scripts/deps.sh'"

test: deps
	@sh -c "'$(CURDIR)/scripts/test.sh'"
	@$(MAKE) vet

cover: deps
	go list ./... | xargs -n1 go test --cover

format: deps
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

vet:
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@echo "--> Running go tool vet $(VETARGS) ."
	@go tool vet $(VETARGS) . ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "[LINT] Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
	fi

	@git grep -n `echo "log"".Print"` ; if [ $$? -eq 0 ]; then \
		echo "[LINT] Found "log"".Printf" calls. These should use Nomad's logger instead."; \
	fi

web:
	./scripts/website_run.sh

web-push:
	./scripts/website_push.sh

# bootstrap the build by downloading additional tools
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing $$tool" ; \
    go get $$tool; \
	done

.PHONY: all bin cov deps integ test vet web web-push test-nodep

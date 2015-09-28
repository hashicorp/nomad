DEPS = $(shell go list -f '{{range .TestImports}}{{.}} {{end}}' ./...)
PACKAGES = $(shell go list ./...)
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
EXTERNAL_TOOLS=\
	github.com/tools/godep \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	golang.org/x/tools/cmd/vet


all: deps format
	@mkdir -p bin/
	@bash --norc -i ./scripts/build.sh

bin:
	@sh -c "'$(CURDIR)/scripts/build.sh'"

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

deps:
	@echo "--> Installing build dependencies"
	@go get -d -v ./... $(DEPS)

updatedeps: deps
	@echo "--> Updating build dependencies"
	@go get -d -f -u ./... $(DEPS)

test: deps
	@./scripts/test.sh
	@$(MAKE) vet

integ:
	go list ./... | INTEG_TESTS=yes xargs -n1 go test

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
		echo "Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for reviewal."; \
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

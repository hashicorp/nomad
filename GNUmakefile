PACKAGES = $(shell go list ./... | grep -v '/vendor/')
EXTERNAL_TOOLS=\
	       github.com/kardianos/govendor \
	       github.com/mitchellh/gox \
	       golang.org/x/tools/cmd/cover \
	       github.com/axw/gocov/gocov \
	       gopkg.in/matm/v1/gocov-html \
	       github.com/ugorji/go/codec/codecgen \
	       github.com/jteeuwen/go-bindata/... \
	       github.com/elazarl/go-bindata-assetfs/...

TEST_TOOLS=\
	   github.com/hashicorp/vault

all: test

dev: format generate
	@scripts/build-dev.sh

bin: generate
	@sh -c "'$(PWD)/scripts/build.sh'"

release: ui generate
	@sh -c "TARGETS=release '$(PWD)/scripts/build.sh'"

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test:
	@if [ ! $(SKIP_NOMAD_TESTS) ]; then \
		make test-nomad; \
		fi
	@if [ $(RUN_UI_TESTS) ]; then \
		make test-ui; \
		fi

test-nomad: generate
	@echo "--> Running go fmt" ;
	@if [ -n "`go fmt ${PACKAGES}`" ]; then \
		echo "[ERR] go fmt updated formatting. Please commit formatted code first."; \
		exit 1; \
		fi
	@sh -c "'$(PWD)/scripts/test.sh'"
	@$(MAKE) vet

test-ui:
	@echo "--> Installing JavaScript assets"
	@cd ui && yarn install
	@cd ui && npm install phantomjs-prebuilt
	@echo "--> Running ember tests"
	@cd ui && phantomjs --version
	@cd ui && npm test

cover:
	go list ./... | xargs -n1 go test --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

generate:
	@echo "--> Running go generate"
	@go generate $(PACKAGES)
	@sed -i.old -e 's|github.com/hashicorp/nomad/vendor/github.com/ugorji/go/codec|github.com/ugorji/go/codec|' nomad/structs/structs.generated.go

vet:
	@echo "--> Running go vet $(VETARGS) ${PACKAGES}"
	@go vet $(VETARGS) ${PACKAGES} ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "[LINT] Vet found suspicious constructs. Please check the reported constructs"; \
		echo "and fix them if necessary before submitting the code for review."; \
		exit 1; \
		fi
	@git grep -n `echo "log"".Print"` | grep -v 'vendor/' ; if [ $$? -eq 0 ]; then \
		echo "[LINT] Found "log"".Printf" calls. These should use Nomad's logger instead."; \
		fi

# bootstrap the build by downloading additional tools
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing $$tool" ; \
		go get $$tool; \
		done
	@for tool in  $(TEST_TOOLS) ; do \
		echo "Installing $$tool (test dependency)" ; \
		go get $$tool; \
		done

install: bin/nomad
	install -o root -g wheel -m 0755 ./bin/nomad /usr/local/bin/nomad

travis:
	@sh -c "'$(PWD)/scripts/travis.sh'"

static-assets:
	@echo "--> Generating static assets"
	@go-bindata-assetfs -pkg agent -prefix ui -modtime 1480000000 -tags ui ./ui/dist/...
	@mv bindata_assetfs.go command/agent

ember-dist:
	@echo "--> Installing JavaScript assets"
	@cd ui && yarn install
	@cd ui && npm rebuild node-sass
	@echo "--> Building Ember application"
	@cd ui && npm run build

ui: ember-dist static-assets

dev-ui: ui format generate
	@scripts/build-dev.sh -ui

.PHONY: all bin cov integ test vet test-nodep static-assets ember-dist static-dist

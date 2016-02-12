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

dev: format
	@NOMAD_DEV=1 sh -c "'$(PWD)/scripts/build.sh'"

bin:
	@sh -c "'$(PWD)/scripts/build.sh'"

release: 
	@$(MAKE) bin

cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	open /tmp/coverage.html

test: 
	@sh -c "'$(PWD)/scripts/test.sh'"
	@$(MAKE) vet

cover: 
	go list ./... | xargs -n1 go test --cover

format:
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

prepare_docker:
	sudo stop docker
	sudo rm -rf /var/lib/docker
	sudo rm -f `which docker`
	sudo apt-key adv --keyserver hkp://p80.pool.sks-keyservers.net:80 --recv-keys 58118E89F3A912897C070ADBF76221572C52609D
	echo "deb https://apt.dockerproject.org/repo ubuntu-trusty main" | sudo tee /etc/apt/sources.list.d/docker.list
	sudo apt-get update
	sudo apt-get install docker-engine=$(DOCKER_VERSION)-0~$(shell lsb_release -cs) -y --force-yes

.PHONY: all bin cov integ test vet web web-push test-nodep

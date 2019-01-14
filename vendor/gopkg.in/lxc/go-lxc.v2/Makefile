NO_COLOR=\033[0m
OK_COLOR=\033[0;32m

all: format vet lint

format:
	@echo "$(OK_COLOR)==> Formatting the code $(NO_COLOR)"
	@gofmt -s -w *.go
	@goimports -w *.go || true

test:
	@echo "$(OK_COLOR)==> Running tests for priveleged user $(NO_COLOR)"
	@sudo `which go` test -v -coverprofile=/tmp/priv.out

test-race:
	@echo "$(OK_COLOR)==> Running tests with -race flag for priveleged user $(NO_COLOR)"
	@sudo `which go` test -race -v

test-unprivileged:
	@echo "$(OK_COLOR)==> Running tests for unprivileged user $(NO_COLOR)"
	@`which go` test -v -coverprofile=/tmp/unpriv.out

test-unprivileged-race:
	@echo "$(OK_COLOR)==> Running tests with -race flag for unprivileged user $(NO_COLOR)"
	@`which go` test -race -v

cover:
	@echo "$(OK_COLOR)==> Running cover for privileged user $(NO_COLOR)"
	@`which go` tool cover -func=/tmp/priv.out || true
	@echo "$(OK_COLOR)==> Running cover for unprivileged user $(NO_COLOR)"
	@`which go` tool cover -func=/tmp/unpriv.out || true

doc:
	@`which godoc` gopkg.in/lxc/go-lxc.v2 | less

vet:
	@echo "$(OK_COLOR)==> Running go vet $(NO_COLOR)"
	@`which go` vet .

lint:
	@echo "$(OK_COLOR)==> Running golint $(NO_COLOR)"
	@`which golint` . || true

escape-analysis:
	@`which go` build -gcflags -m

ctags:
	@ctags -R --languages=c,go

scope:
	@echo "$(OK_COLOR)==> Exported container calls in container.go $(NO_COLOR)"
	@/bin/grep -E "\bc+\.([A-Z])\w+" container.go || true

setup-test-cgroup:
	for d in /sys/fs/cgroup/*; do \
	    [ -f $$d/cgroup.clone_children ] && echo 1 | sudo tee $$d/cgroup.clone_children; \
	    [ -f $$d/cgroup.use_hierarchy ] && echo 1 | sudo tee $$d/cgroup.use_hierarchy; \
	    sudo mkdir -p $$d/lxc; \
	    sudo chown -R $$USER: $$d/lxc; \
	done

.PHONY: all format test doc vet lint ctags

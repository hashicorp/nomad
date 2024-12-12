# set shell := ["bash", "-u", "-c"]

export CGO_ENABLED := '1'
export git_meta := '' # +ent for enterprise
export go_os := shell('go env GOOS')
export go_arch := shell('go env GOARCH')
export go_cc := shell('go env CC')
export go_module := 'github.com/hashicorp/nomad'
export build_date := shell('TZ=UTC0 git show -s --format=%cd --date=format-local:"%Y-%m-%dT%H:%M:%SZ" HEAD')
export build_sha := shell('git rev-parse HEAD')
export build_dirty := shell('if [ -n "$(git status --porcelain)" ]; then echo "+CHANGES"; fi')
go_tags := if git_meta =~ '\+ent' { env('GO_TAGS', 'ui,ent') } else { env('GO_TAGS', 'ui') }
go_target_pkg := 'pkg/${go_os}_${go_arch}/nomad'
go_target_bin := env('GOBIN', 'bin') + '/nomad'
go_ldflags := '-X ${go_module}/version.GitCommit=${build_sha}${build_dirty} -X ${go_module}/version.BuildDate=${build_date}'
git_directory := shell('git rev-parse --git-dir')
ext := '' # overrides to .exe on windows

# our understanding of GOBIN could come from
# - environment variable (direct)
# - environment file (use go env)
# - default to gopath (on PATH in CI)
export go_bin := if env('GOBIN', '') != '' {
    env('GOBIN')
} else if shell('go env GOBIN') != '' {
    shell('go env GOBIN')
}else {
    shell('go env GOPATH') / 'bin'
}

# proto_compare_tag is used to mark the release where we maintain protobuf
# backwards compatibility; we should not make breaking changes from here.
proto_compare_tag := 'v1.0.3$git_meta'

# last_release is used for generating the changelog. It is the last released GA
# or backport version, without the leading "v". main should have the latest
# published release here, and release branches should point to the latest
# published release in their X.Y release line.
last_release := '1.9.3'

# List available Justfile targets.
@default:
    just --list

# Install all dependencies and setup githooks.
[group('setup')]
@bootstrap:
    just deps
    just lint-deps
    just git-hooks

# Create a new changelog entry.
[group('tools')]
@cl:
    go run -modfile tools/go.mod tools/cl-entry/main.go

# Create changelog from entries.
[group('tools')]
@changelog:
    changelog-build -last-release v{{last_release}} -this-release HEAD \
        -entries-dir .changelog -changelog-template ./.changelog/changelog.tmpl \
        -note-template ./.changelog/note.tmpl

# Install build dependencies.
[group('setup')]
@deps:
    echo "==> Updating build dependencies..."
    go install github.com/hashicorp/go-bindata/go-bindata@bf7910af899725e4938903fb32048c7c0b15f12e
    go install github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs@234c15e7648ff35458026de92b34c637bae5e6f7
    go install github.com/a8m/tree/cmd/tree@fce18e2a750ea4e7f53ee706b1c3d9cbb22de79c
    go install gotest.tools/gotestsum@v1.10.0
    go install github.com/hashicorp/hcl/v2/cmd/hclfmt@d0c4fa8b0bbc2e4eeccd1ed2a32c2089ed8c5cf1
    go install github.com/golang/protobuf/protoc-gen-go@v1.3.4
    go install github.com/hashicorp/go-msgpack/v2/codec/codecgen@v2.1.2
    go install github.com/bufbuild/buf/cmd/buf@v0.36.0
    go install github.com/hashicorp/go-changelog/cmd/changelog-build@latest
    go install golang.org/x/tools/cmd/stringer@v0.18.0
    go install github.com/hashicorp/hc-install/cmd/hc-install@v0.9.0
    go install github.com/shoenig/go-modtool@v0.2.0

# Install linter dependencies.
[group('setup')]
@lint-deps:
    echo "==> Updating linter dependencies..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
    go install github.com/client9/misspell/cmd/misspell@v0.3.4
    go install github.com/hashicorp/go-hclog/hclogvet@v0.2.0

# Generate structs, protobufs, copywrite.
[group('generate')]
@generate-all:
    just proto
    just generate-structs
    -just copywriteheaders
    # TODO: always fails when running locally (??)

# Execute 'go generate' on Go files.
[group('generate')]
@generate-structs:
    echo "==> Execute go generate on source files ..."
    go generate $(go list ./...)

# Write BUSL copywrite headers.
[group('generate')]
@copywriteheaders:
    copywrite headers --plan
    cd api && $(CURDIR)/scripts/copywrite-exceptions.sh
    cd drivers/shared && $(CURDIR)/scripts/copywrite-exceptions.sh
    cd plugins && $(CURDIR)/scripts/copywrite-exceptions.sh
    cd jobspec2 && $(CURDIR)/scripts/copywrite-exceptions.sh
    cd demo && $(CURDIR)/scripts/copywrite-exceptions.sh

# Generate protobufs.
[group('generate')]
@proto:
    echo "==> Generating proto bindings ..."
    buf --config tools/buf/buf.yaml --template tools/buf/buf.gen.yaml generate

# Lint protobuf files.
[group('lint')]
@checkproto:
    just proto

    echo "==> Lint proto files ..."
    -buf lint --config tools/buf/buf.yaml
    # TODO we have linter violations; ignore return code for now

    echo "==> Checking for breaking changes in protos ..."
    buf breaking --config tools/buf/buf.yaml --against-config tools/buf/buf.yaml --against .git#tag={{proto_compare_tag}}

# Lint script files.
[group('lint')]
@checkscripts:
    echo "==> Linting scripts ..."
    -find scripts -type f -name '*.sh' | xargs shellcheck
    # TODO we have linter violations; ignore return code for now

# Lint the source code.
[group('lint')]
@check:
    echo "==> Linting source code ..."
    golangci-lint run --build-tags {{go_tags}}

    echo "==> Linting ./api source code ..."
    cd api && golangci-lint run --config ../.golangci.yml --build-tags {{go_tags}}

    echo "==> Linting hclog statements ..."
    hclogvet .

    echo "==> Spell checking website ..."
    misspell -error -source=text website/content/

    just checkproto
    just checkscripts

# Install CNI plugins.
[group('setup')]
@cni:
    mkdir -p /opt/cni/bin
    curl --fail -LsO 'https://github.com/containernetworking/plugins/releases/download/v1.3.0/cni-plugins-linux-amd64-v1.3.0.tgz'
    tar -C /opt/cni/bin -xf 'cni-plugins-linux-amd64-v1.3.0.tgz'

# Install local git hooks.
[group('setup')]
@git-hooks:
    cp dev/hooks/* "{{git_directory}}/hooks/"
    chmod 755 "{{git_directory}}/hooks"

# Format HCL files using hcltfmt.
[group('build')]
@hclfmt:
    echo "==> Formatting HCL ..."
    find . -name '.terraform' -prune \
        -o -name 'upstart.nomad' -prune \
        -o -name '.git' -prune \
        -o -name 'node_modules' -prune \
        -o -name '.next' -prune \
        -o -path './ui/dist' -prune \
        -o -path './website/out' -prune \
        -o -path './command/testdata' -prune \
        -o \( -name '*.nomad' -o -name '*.hcl' -o -name '*.tf' \) \
            -print0 | xargs -0 hclfmt -w

# Cleanup previous build steps.
[group('build')]
@clean:
    echo "==> Removing old build(s) ..."
    rm -f {{go_target_pkg}}
    rm -f {{go_target_bin}}
    rm -f {{go_bin}}/nomad

# Compile development build.
[group('build')]
@dev: clean hclfmt
    echo "==> Compiling development build ..."
    go build -o {{go_target_pkg}} -trimpath -ldflags '{{go_ldflags}}' -tags '{{go_tags}}'
    mkdir -p bin
    cp {{go_target_pkg}} {{go_target_bin}}
    cp {{go_target_pkg}} {{go_bin}}/nomad

# Tidy up the go mod files.
[group('build')]
@tidy:
    echo "==> Tidy up submodules"
    cd tools && go mod tidy
    cd api && go mod tidy
    echo "==> Tidy nomad module"
    go-modtool -config=ci/modtool.toml fmt go.mod
    go mod tidy

# Generate static routes to serve alongside the API.
[group('generate')]
@static-assets:
    echo "==> Generating static assets ..."
    go-bindata-assetfs -pkg agent -prefix ui -modtime 1480000000 -tags ui -o bindata_assetfs.go ./ui/dist/...
    mv bindata_assetfs.go command/agent

# Generate static ember UI assets from source.
[group('generate')]
@ember-dist:
    echo "==> Installing JavaScript assets ..."
    cd ui && yarn install --silent --network-timeout 300000
    cd ui && npm rebuild node-sass
    echo "==> Building Ember application"
    cd ui && npm run build

# Check for packages not being tested.
[group('tools')]
@missing:
    echo "==> Checking for packages not being tested ..."
    go run -modfile tools/go.mod tools/missing/main.go ci/test-core.json

# Print current build version.
[group('tools')]
@version:
    if [[ ${git_meta} == '*+ent' ]]; then \
        ./scripts/version.sh version/version.go version/version_ent.go; \
    else \
        ./scripts/version.sh version/version.go version/version.go; \
    fi

# Test Nomad package by specific group.
[group('testing')]
@test-nomad group:
    echo "==> Running Unit tests for group {{group}}"
    gotestsum \
        --format=testname \
        --rerun-fails=3 \
        --packages=$(go run -modfile=tools/go.mod tools/missing/main.go ci/test-core.json {{group}}) \
        -- \
        -cover \
        -timeout=20m \
        -tags {{go_tags}} \
        $(go run -modfile=tools/go.mod tools/missing/main.go ci/test-core.json {{group}})

# Test Nomad sub-module.
[group('testing')]
@test-nomad-sub module: dev
    echo "==> Running Nomad unit test on submodule {{module}} ..."
    cd {{module}}; gotestsum \
        --format=testname \
        --rerun-fails=3 \
        --packages=./... \
        -- \
        -cover \
        -timeout=20m \
        -count=1 \
        -race \
        -tags {{go_tags}} \
        ./...

# Generate all the things for release.
[group('release')]
@prerelease:
    export go_tags='ui,codegen_generated,release'
    just generate-all
    just ember-dist
    just static-assets

# Compile a specific os/arch target.
[private]
@compile:
    printf "compiling nomad ...\n"
    printf "\tGOOS ->   {{go_os}}\n"
    printf "\tGOARCH -> {{go_arch}}\n"
    printf "\tCC ->     {{go_cc}}\n"
    printf "\tbinary -> nomad{{ext}}\n"
    printf "\ttags ->   {{go_tags}}\n"

    CC={{go_cc}} GOOS={{go_os}} GOARCH={{go_arch}} \
    go build \
        -trimpath \
        -ldflags '{{go_ldflags}}' \
        -tags {{go_tags}} \
        -o pkg/{{go_os}}_{{go_arch}}/nomad{{ext}}

# Build all release artifacts.
[group('release')]
release: clean
    #!/usr/bin/env -S bash -euo pipefail
    export go_tags='ui,codegen_generated,release'
    case {{go_os}} in
        linux)
           just go_os=linux go_arch=amd64 compile
           just go_os=linux go_arch=arm64 go_cc=aarch64-linux-gnu-gcc compile
           just go_os=linux go_arch=s390x compile
           just go_os=windows go_arch=amd64 ext=.exe compile
           ;;
        darwin)
           just go_os=linux go_arch=amd64 compile
           just go_os=linux go_arch=arm64 compile
           ;;
        freebsd)
           just go_os=freebsd go_arch=amd64 compile
           ;;
        *)
           echo "unnacceptable operating system"
           exit 1
           ;;
    esac

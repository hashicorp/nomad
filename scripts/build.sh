#!/usr/bin/env bash
#
# This script builds the application from source for multiple platforms.
set -e

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that directory
cd "$DIR"

# Get the git commit
GIT_COMMIT="$(git rev-parse HEAD)"
GIT_DIRTY="$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)"
LDFLAG="main.GitCommit=${GIT_COMMIT}${GIT_DIRTY}"

# Delete the old dir
echo "==> Removing old directory..."
rm -f bin/*
rm -rf pkg/*
mkdir -p bin/

if [[ $(uname) == "Linux" ]]; then
    echo "==> Building linux 386..."
    CGO_ENABLED=1 GOARCH="386"   GOOS="linux"   go build -ldflags "-X $LDFLAG" -o "pkg/linux_386/nomad"

    echo "==> Building linux amd64..."
    CGO_ENBALED=1 GOARCH="amd64" GOOS="linux"   go build -ldflags "-X $LDFLAG" -o "pkg/linux_amd64/nomad"

    echo "==> Building linux amd64 with lxc..."
    CGO_ENBALED=1 GOARCH="amd64" GOOS="linux"   go build -ldflags "-X $LDFLAG" -o "pkg/linux_amd64-lxc/nomad" -tags "lxc"

    echo "==> Building linux arm..."
    CC="arm-linux-gnueabi-gcc-5" GOOS=linux GOARCH="arm"   CGO_ENABLED=1 go build -ldflags "-X $LDFLAG" -o "pkg/linux_arm/nomad"

    echo "==> Building linux arm64..."
    CC="aarch64-linux-gnu-gcc-5"  GOOS=linux GOARCH="arm64" CGO_ENABLED=1 go build -ldflags "-X $LDFLAG" -o "pkg/linux_arm64/nomad"

    echo "==> Building windows 386..."
    CGO_ENABLED=1 GOARCH="386"   GOOS="windows" go build -ldflags "-X $LDFLAG" -o "pkg/windows_386/nomad"

    echo "==> Building windows amd64..."
    CGO_ENABLED=1 GOARCH="amd64" GOOS="windows" go build -ldflags "-X $LDFLAG" -o "pkg/windows_amd64/nomad"
elif [[ $(uname) == "Darwin" ]]; then
    echo "==> Building darwin amd64..."
    CGO_ENABLED=1 GOARCH="amd64" GOOS="darwin"  go build -ldflags "-X $LDFLAG" -o "pkg/darwin_amd64/nomad"
else
    echo "Unable to build on $(uname). Use Linux or Darwin."
    exit 1
fi

# Move all the compiled things to the $GOPATH/bin
GOPATH=${GOPATH:-$(go env GOPATH)}
case $(uname) in
    CYGWIN*)
        GOPATH="$(cygpath $GOPATH)"
        ;;
esac
OLDIFS=$IFS
IFS=: MAIN_GOPATH=($GOPATH)
IFS=$OLDIFS

# Copy our OS/Arch to the bin/ directory
DEV_PLATFORM="./pkg/$(go env GOOS)_$(go env GOARCH)"
for F in $(find ${DEV_PLATFORM} -mindepth 1 -maxdepth 1 -type f); do
    cp ${F} bin/
    cp ${F} ${MAIN_GOPATH}/bin/
done

# Zip and copy to the dist dir
echo "==> Packaging..."
for PLATFORM in $(find ./pkg -mindepth 1 -maxdepth 1 -type d); do
    OSARCH=$(basename ${PLATFORM})
    echo "--> ${OSARCH}"

    pushd $PLATFORM >/dev/null 2>&1
    zip ../${OSARCH}.zip ./*
    popd >/dev/null 2>&1
done

# Done!
echo
echo "==> Results:"
tree pkg/

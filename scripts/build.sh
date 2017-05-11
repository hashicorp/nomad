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

targets="$(go env GOOS)_$(go env GOARCH)"
if [[ "$TARGETS" == "release" ]]; then
    if [[ $(uname) == "Linux" ]]; then
        targets="linux_386 linux_amd64 linux_amd64-lxc linux_arm linux_arm64 windows_386 windows_amd64"
    elif [[ $(uname) == "Darwin" ]]; then
	targets="darwin_amd64"
    else
        echo "Unable to build on $(uname). Use Linux or Darwin."
        exit 1
    fi
elif [[ "$TARGETS" != "" ]]; then
    targets="$TARGETS"
fi

# Don't exit if a single target fails
set +e

echo "TARGETS=\"$targets\""
for target in $targets; do
    case $target in
        "linux_386")
            echo "==> Building linux 386..."
            CGO_ENABLED=1 GOARCH="386"   GOOS="linux" go build -ldflags "-X $LDFLAG" -o "pkg/linux_386/nomad"
            ;;
        "linux_amd64")
            echo "==> Building linux amd64..."
            CGO_ENABLED=1 GOARCH="amd64" GOOS="linux" go build -ldflags "-X $LDFLAG" -o "pkg/linux_amd64/nomad"
            ;;
        "linux_amd64-lxc")
            echo "==> Building linux amd64 with lxc..."
            CGO_ENABLED=1 GOARCH="amd64" GOOS="linux" go build -ldflags "-X $LDFLAG" -o "pkg/linux_amd64-lxc/nomad" -tags "lxc"
            ;;
        "linux_arm")
            echo "==> Building linux arm..."
            CGO_ENABLED=1 CC="arm-linux-gnueabihf-gcc-5" GOOS=linux GOARCH="arm"  go build -ldflags "-X $LDFLAG" -o "pkg/linux_arm/nomad"
            ;;
        "linux_arm64")
            echo "==> Building linux arm64..."
            CGO_ENABLED=1 CC="aarch64-linux-gnu-gcc-5"  GOOS=linux GOARCH="arm64" go build -ldflags "-X $LDFLAG" -o "pkg/linux_arm64/nomad"
            ;;
        "windows_386")
            echo "==> Building windows 386..."
            CGO_ENABLED=0 GOARCH="386"   GOOS="windows" go build -ldflags "-X $LDFLAG" -o "pkg/windows_386/nomad.exe"
            # Use the following if CGO is required
            #CGO_ENABLED=1 CXX=i686-w64-mingw32-g++ CC=i686-w64-mingw32-gcc GOARCH="386"   GOOS="windows" go build -ldflags "-X $LDFLAG" -o "pkg/windows_386/nomad.exe"
            ;;
        "windows_amd64")
            echo "==> Building windows amd64..."
            CGO_ENABLED=0 GOARCH="amd64" GOOS="windows" go build -ldflags "-X $LDFLAG" -o "pkg/windows_amd64/nomad.exe"
            # Use the following if CGO is required
            #CGO_ENABLED=1 CXX=x86_64-w64-mingw32-g++ CC=x86_64-w64-mingw32-gcc GOARCH="amd64" GOOS="windows" go build -ldflags "-X $LDFLAG" -o "pkg/windows_amd64/nomad.exe"
            ;;
        "darwin_amd64")
            echo "==> Building darwin amd64..."
            CGO_ENABLED=1 GOARCH="amd64" GOOS="darwin"  go build -ldflags "-X $LDFLAG" -o "pkg/darwin_amd64/nomad"
            ;;
        *)
            echo "--> Invalid target: $target"
            ;;
    esac
done

set -e

# Move all the compiled things to $GOPATH/bin
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

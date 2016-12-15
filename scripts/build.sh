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

# Determine the arch/os combos we're building for
# XC_ARCH=${XC_ARCH:-"386 amd64"}
# XC_OS=${XC_OS:-linux}

XC_ARCH=${XC_ARCH:-"386 amd64"}
XC_OS=${XC_OS:-"linux"}
XC_EXCLUDE=${XC_EXCLUDE:-"!darwin/arm !darwin/386"}

# Delete the old dir
echo "==> Removing old directory..."
rm -f bin/*
rm -rf pkg/*
mkdir -p bin/

# If its dev mode, only build for ourself
if [[ "${NOMAD_DEV}" ]]; then
    XC_OS=$(go env GOOS)
    XC_ARCH=$(go env GOARCH)
fi

# Build!
echo "==> Building..."
gox \
    -os="${XC_OS}" \
    -arch="${XC_ARCH}" \
    -osarch="${XC_EXCLUDE}" \
    -cgo \
    -ldflags "-X main.GitCommit='${GIT_COMMIT}${GIT_DIRTY}'" \
    -output "pkg/{{.OS}}_{{.Arch}}/nomad" \
    .

echo ""
if pkg-config --exists lxc; then
    echo "==> Building linux_amd64-lxc..."
    go build \
        -tags lxc \
        -ldflags "-X main.GitCommit='${GIT_COMMIT}${GIT_DIRTY}+lxc'" \
        -o "pkg/linux_amd64-lxc/nomad"
else
    if [[ "${NOMAD_DEV}" ]]; then
	 # No lxc in dev mode is no problem
        echo "LXC not installed; skipping"
    else
	# Require LXC for release mode
	echo "LXC not installed; install lxc-dev to build release binaries"
	exit 1
    fi
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

if [[ "x${NOMAD_DEV}" == "x" ]]; then
    # Zip and copy to the dist dir
    echo "==> Packaging..."
    for PLATFORM in $(find ./pkg -mindepth 1 -maxdepth 1 -type d); do
        OSARCH=$(basename ${PLATFORM})
        echo "--> ${OSARCH}"

        pushd $PLATFORM >/dev/null 2>&1
        zip ../${OSARCH}.zip ./*
        popd >/dev/null 2>&1
    done
fi

# Done!
echo
echo "==> Results:"
ls -hl bin/

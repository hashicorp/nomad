#!/bin/sh

if [ -z "$1" ]; then
    echo "usage: $0 GO_VERSION"
    echo ""
    echo "For example: $0 1.15.5"
    exit 1
fi

golang_version="$1"

current_version=$(grep -o -E -e '/golang:[.0-9]+' .circleci/config.yml | head -n1 | cut -d: -f2)
if [ -z "${current_version}" ]; then
    echo "unable to find current go version"
    exit 1
fi
echo "--> Replacing Go ${current_version} with Go ${golang_version} ..."

# To support both GNU and BSD sed, the regex is looser than it needs to be.
# Specifically, we use "* instead of "?, which relies on GNU extension without much loss of
# correctness in practice.
sed -i'' -e "s|/golang:[.0-9]*|/golang:${golang_version}|g" .circleci/config.yml
sed -i'' -e "s|GOLANG_VERSION:[ \"]*[.0-9]*\"*|GOLANG_VERSION: ${golang_version}|g" \
	.circleci/config.yml

sed -i'' -e "s|\\(Install .Go\\) [.0-9]*|\\1 ${golang_version}|g" \
	contributing/README.md

sed -i'' -e "s|go_version=\"*[^\"]*\"*$|go_version=\"${golang_version}\"|g" \
	scripts/vagrant-linux-priv-go.sh scripts/release/mac-remote-build

echo "--> Checking if there is any remaining references to old versions..."
if git grep -I --fixed-strings "${current_version}" | grep -v -e CHANGELOG.md -e vendor/ -e website/ -e ui/ -e contributing/golang.md -e '.*.go:' -e go.sum -e go.mod  -e LICENSE
then
	echo "  ^^ files may contain references to old golang version" >&2
	echo "  update script and run again" >&2
	exit 1
fi

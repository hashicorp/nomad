#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

set -e

# Get the version from the command line
VERSION=$1
if [ -z "${VERSION}" ]; then
  echo "Please specify a version. (format: 0.4.0-rc1)"
  exit 1
fi

gpg_signing_key=348FFC4C

verify_hc_releases() {
    if ! command -v  hc-releases 2>/dev/null >/dev/null
    then
        echo "hc-releases binary is not present" >&2
        exit 1
    fi
}

verify_gpg_key() {
    if ! gpg --list-keys "${gpg_signing_key}" >/dev/null 2>/dev/null
    then
        echo "gpg key ${gpg_signing_key} is not present" >&2
        exit 1
    fi
}

verify_s3_access() {
    if ! aws s3 ls s3://hc-releases/ >/dev/null 2>/dev/null
    then
        echo "AWS credentials is not configured" >&2
        exit 1
    fi
}

if [ -z "${NO_PREFLIGHT}" ]; then
    verify_hc_releases
    verify_gpg_key
    verify_s3_access
fi

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that
cd "${DIR}"

# Generate the tag.
if [ -z "${NOTAG}" ]; then
  echo "==> Tagging..."
  git commit --allow-empty -a --gpg-sign=${gpg_signing_key} -m "Release v$VERSION"
  git tag -a -m "Version $VERSION" -s -u ${gpg_signing_key} "v${VERSION}" HEAD
fi

# Zip all the files
rm -rf ./pkg/dist
mkdir -p ./pkg/dist

#find ./pkg -mindepth 1 -maxdepth 1 -type f -exec cp ./pkg/{} ./pkg/dist/nomad_"${VERSION}"_{} \;
#for FILENAME in $(find ./pkg -mindepth 1 -maxdepth 1 -type f); do
find ./pkg -mindepth 1 -maxdepth 1 -type f -print0 | while read -d '' -r FILENAME; do
  FILENAME=$(basename "$FILENAME")
  cp "./pkg/${FILENAME}" "./pkg/dist/nomad_${VERSION}_${FILENAME}"
done

# Make the checksums
pushd ./pkg/dist
shasum -a256 ./* > "./nomad_${VERSION}_SHA256SUMS"
if [ -z "${NOSIGN}" ]; then
  echo "==> Signing..."
  gpg --default-key 348FFC4C --detach-sig "./nomad_${VERSION}_SHA256SUMS"
fi
popd

# Upload
if [ -z "${HC_RELEASE}" ]; then
  hc-releases upload "${DIR}/pkg/dist" && hc-releases publish
fi

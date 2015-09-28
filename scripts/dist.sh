#!/usr/bin/env bash
set -e

# Get the version from the command line
VERSION=$1
if [ -z $VERSION ]; then
    echo "Please specify a version."
    exit 1
fi

# Make sure we have a bintray API key
if [ -z $BINTRAY_API_KEY ] && [ ! -z $NOBINTRAY ]; then
    echo "Please set your bintray API key in the BINTRAY_API_KEY env var."
    exit 1
fi

# Get the parent directory of where this script is.
SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ] ; do SOURCE="$(readlink "$SOURCE")"; done
DIR="$( cd -P "$( dirname "$SOURCE" )/.." && pwd )"

# Change into that dir because we expect that
cd $DIR

# Zip all the files
rm -rf ./pkg/dist
mkdir -p ./pkg/dist
for FILENAME in $(find ./pkg -mindepth 1 -maxdepth 1 -type f); do
    FILENAME=$(basename $FILENAME)
    cp ./pkg/${FILENAME} ./pkg/dist/nomad_${VERSION}_${FILENAME}
done

if [ -z $NOSIGN ]; then
  echo "==> Signing..."
  pushd ./pkg/dist
  rm -f ./nomad_${VERSION}_SHA256SUMS*
  shasum -a256 * > ./nomad_${VERSION}_SHA256SUMS
  gpg --default-key 348FFC4C --detach-sig ./nomad_${VERSION}_SHA256SUMS
  popd
fi

# Upload
if [ ! -z $NOBINTRAY ]; then
  for ARCHIVE in ./pkg/dist/*; do
    ARCHIVE_NAME=$(basename ${ARCHIVE})

    echo Uploading: $ARCHIVE_NAME
    curl \
        -T ${ARCHIVE} \
        -umitchellh:${BINTRAY_API_KEY} \
        "https://api.bintray.com/content/mitchellh/nomad/nomad/${VERSION}/${ARCHIVE_NAME}"
  done
fi

exit 0

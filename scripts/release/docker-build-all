#!/usr/bin/env bash

# A script for building Linux and Windows nomad release binaries inside Docker
#
# This is a helper script file that is expected to be invoked
# within a docker container with an image created with the Dockerfile present on this directory.
#
# A sample way of invoking the script is
# ```
# docker run --rm \
#        -v $(pwd)/.git:/tmp/nomad-git:ro \
#        -v /tmp/generated-repo:/tmp/artifacts:rw \
#        -e "PRERELEASE_TARGET=${PRERELEASE_TARGET}" \
#        -e "RELEASE_TARGET=${RELEASE_TARGET}" \
#        -e "NOMAD_VERSION=${NOMAD_VERSION}" \
#        nomad-builder:latest \
#        /tmp/scripts/docker-build-all
# ```
# Namely the script takes the following arguments:
# * `/tmp/nomad-git` path being a read-only .git directory with HEAD being the sha to be released
# * `NOMAD_VERSION` env-var being the release version to be cut (e.g. `0.9.1-rc1`)
# * `PRERELEASE_TARGET` env-var being the prerelease make target, typically `prerelease`.  Use `help` to skip `prerelease` step
# * `RELEASE_TARGET` env-var being the release make target, typically `release`.
#
#
# The script would then run prerelease steps, commits any generated files, and build all binary files
# and stores them to `/tmp/artifacts/repo`.

set -o errexit
set -o xtrace

cp -r /tmp/nomad-git /opt/gopath/src/github.com/hashicorp/nomad/.git

cd /opt/gopath/src/github.com/hashicorp/nomad

# checkout directory from .git and ensures a prestine state
git checkout .

make -f ./scripts/release/Makefile.linux \
  "NOMAD_VERSION=${NOMAD_VERSION}" \
  "PRERELEASE_TARGET=${PRERELEASE_TARGET}" \
  "RELEASE_TARGET=${RELEASE_TARGET}" \
  update_version build_releases

cp -r /opt/gopath/src/github.com/hashicorp/nomad \
    /tmp/artifacts/repo

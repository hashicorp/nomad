#!/bin/bash

set -e

docker build -f dev/docker-dev-env/Dockerfile -t nomad-dev-env .
docker run --rm -ti -v /tmp:/tmp nomad-dev-env bash -c "cp /output/nomad /tmp/"
adb push /tmp/nomad /system/xbin/

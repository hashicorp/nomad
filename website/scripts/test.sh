#!/bin/bash
set -e

MMVERSION=${1-0.3.35}

WEBSITE_CID=$(docker run \
    --detach \
    --rm \
    --publish "4567:4567" \
    --publish "35729:35729" \
    --volume "$PWD:/website" \
    hashicorp/middleman-hashicorp:${MMVERSION})

trap 'echo Killing docker website container... && docker kill ${WEBSITE_CID}' EXIT HUP INT QUIT TERM

echo Checking website for dead links...
wget --recursive --delete-after http://localhost:4567

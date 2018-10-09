#!/bin/bash
set -e

# passed in from outer makefile, default to 0.3.35
MMVERSION=${1-0.3.35}

echo "Running website in container..."
WEBSITE_CID=$(docker run \
    --detach \
    --rm \
    --publish "4567:4567" \
    --publish "35729:35729" \
    --volume "$PWD:/website" \
    hashicorp/middleman-hashicorp:${MMVERSION})
echo "Website running in container ${WEBSITE_CID}."

trap 'echo Killing docker website container... && docker kill ${WEBSITE_CID}' EXIT HUP INT QUIT TERM

echo "Checking website for dead links using recursive wget..."
wget -nv --recursive --level=10 --delete-after http://localhost:4567

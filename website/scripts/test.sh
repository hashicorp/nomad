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
echo "Website is launching in container ${WEBSITE_CID}."

echo "Checking website for dead links using recursive wget..."
wget -nv --recursive --level=10 --delete-after http://localhost:4567 && exit_code=$? || exit_code=$?
echo "wget exit code: ${exit_code}"

echo "Killing docker website container ${WEBSITE_CID}..." 
docker kill ${WEBSITE_CID} || true

exit ${exit_code}

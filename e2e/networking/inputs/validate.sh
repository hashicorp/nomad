#!/bin/bash

echo ${NOMAD_ADDR}/v1/allocation/${NOMAD_ALLOC_ID} >> /tmp/out
curl ${NOMAD_ADDR}/v1/allocation/${NOMAD_ALLOC_ID} >> /tmp/out2
test $(curl --silent ${NOMAD_ADDR}/v1/allocation/${NOMAD_ALLOC_ID} | jq '.NetworkStatus.Address | length') -ne 0
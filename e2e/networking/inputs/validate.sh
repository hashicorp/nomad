#!/bin/bash

test $(curl --silent ${NOMAD_ADDR}/v1/allocation/${NOMAD_ALLOC_ID} | jq '.NetworkStatus.Address | length') -ne 0
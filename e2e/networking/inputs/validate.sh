#!/usr/bin/env bash

nomad operator api "/v1/allocation/${NOMAD_ALLOC_ID}" | jq '.NetworkStatus.Address | length'

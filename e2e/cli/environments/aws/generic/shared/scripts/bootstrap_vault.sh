#!/usr/bin/env bash
set -e

initjson=`vault operator init -key-shares=1 -key-threshold=1 -format=json`
unsealkey=`echo $initjson | jq -r .unseal_keys_b64[0]`
vault operator unseal -format=json $unsealkey > /dev/null
root_token=`echo $initjson | jq -r .root_token`
consul kv put .insecure_root_token $root_token


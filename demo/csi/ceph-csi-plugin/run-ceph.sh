#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0



CONSUL_HTTP_ADDR=${CONSUL_HTTP_ADDR:-http://localhost:8500}

echo
echo "nomad job run -var-file=nomad.vars ./ceph.nomad"

nomad job run -var-file=nomad.vars ./ceph.nomad

echo
echo -n "waiting for Ceph to be ready..."
while :
do
    STATUS=$(curl -s "$CONSUL_HTTP_ADDR/v1/health/checks/ceph-dashboard" | jq -r '.[0].Status')
    if [[ "$STATUS" == "passing" ]]; then echo; break; fi
    echo -n "."
    sleep 1
done
echo "ready!"

#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "waiting for Consul leader to be up..."
while true :
do
    consul info && break
    echo "Consul server not ready, waiting 5s"
    sleep 5
done

consul acl bootstrap || echo "Consul ACLs already bootstrapped"

if [ $(consul info | grep -q "version_metadata = ent") ]; then
    echo "writing namespaces"
    consul namespace create -name "prod"
    consul namespace create -name "dev"
fi

echo "writing Nomad cluster policy and token"
consul acl policy create -name nomad-cluster -rules @${DIR}/nomad-cluster-consul-policy.hcl
consul acl token create -policy-name=nomad-cluster -secret "$NOMAD_CLUSTER_CONSUL_TOKEN"

echo "writing Consul cluster policy and token"
consul acl policy create -name consul-agents -rules @${DIR}/consul-agents-policy.hcl
consul acl token create -policy-name=consul-agents -secret "$CONSUL_AGENT_TOKEN"

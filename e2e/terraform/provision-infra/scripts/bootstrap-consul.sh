#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

echo "waiting for Consul leader to be up..."
while true :
do
    pwd
    echo CONSUL_CACERT=$CONSUL_CACERT
    echo CONSUL_HTTP_ADDR=$CONSUL_HTTP_ADDR
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

# The following ACL's are used so Nomad services and tasks can register
# via Workload Identity
echo "writing ACLs for Nomad Workload Identity integration..."

CONF=$(sed 's\NOMAD_SERVER_ADDR\'${NOMAD_SERVER_ADDR}'\g' ${DIR}/consul-workload-identity/auth-method.json) 

echo "writing Consul auth-method"
consul acl auth-method create \
  -name 'nomad-workloads' \
  -type 'jwt' \
  -description 'Login method for Nomad workloads using workload identities' \
  -token-locality 'local' \
  -config "${CONF}" \
  -namespace-rule-selector '"consul_namespace" in value' \
  -namespace-rule-bind-namespace '${value.consul_namespace}'

echo "writing binding-rule for Nomad services"
consul acl binding-rule create \
    -method 'nomad-workloads' \
    -bind-type 'service' \
    -bind-name '${value.nomad_service}' \
    -selector '"nomad_service" in value'

echo "writing binding-rule for Nomad tasks"
consul acl binding-rule create \
  -method 'nomad-workloads' \
  -bind-type 'role' \
  -bind-name 'nomad-tasks-${value.nomad_namespace}' \
  -selector '"nomad_service" not in value'

echo "writing policy for Nomad tasks"
consul acl policy create -name policy-nomad-tasks -rules @${DIR}/consul-workload-identity/nomad-task-policy.hcl

echo "creating role for Nomad tasks using previously created policy"
consul acl role create -name nomad-default-tasks -policy-name policy-nomad-tasks

echo "Consul successfully bootstraped!"

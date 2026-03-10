#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# The following ACL's are used so Nomad services and tasks can register
# via Workload Identity
echo "writing ACLs for Nomad Workload Identity integration..."

# replaces the newlines in the cert with escaped newlines so they are valid JSON
CERT=$(cat ${CONSUL_CACERT} | sed 's/$/\\n/g' | tr -d '\n')

AUTH=$(cat <<EOF
{
    "JWKSURL": "${NOMAD_SERVER_ADDR}/.well-known/jwks.json",
    "JWTSupportedAlgs": [
        "RS256"
    ],
    "JWKSCACert": "${CERT}",
    "BoundAudiences": [
        "consul.io"
    ],
    "ClaimMappings": {
        "consul_namespace": "consul_namespace",
        "nomad_job_id": "nomad_job_id",
        "nomad_namespace": "nomad_namespace",
        "nomad_service": "nomad_service",
        "nomad_task": "nomad_task"
    }
}
EOF
)

echo "writing Consul auth-method"

consul info | grep -q "version_metadata = ent"
if [ $? -eq 0 ]; then
  consul acl auth-method create \
    -name 'nomad-workloads' \
    -type 'jwt' \
    -description 'Login method for Nomad workloads using workload identities' \
    -token-locality 'local' \
    -config "${AUTH}" \
    -namespace-rule-selector '"consul_namespace" in value' \
    -namespace-rule-bind-namespace '${value.consul_namespace}'
else
  consul acl auth-method create \
    -name 'nomad-workloads' \
    -type 'jwt' \
    -description 'Login method for Nomad workloads using workload identities' \
    -token-locality 'local' \
    -config "${AUTH}"
fi

echo "writing binding-rule for Nomad services"
consul acl binding-rule create \
    -method 'nomad-workloads' \
    -description 'Binding rule for Nomad services authenticated using a workload identity' \
    -bind-type 'service' \
    -bind-name '${value.nomad_service}' \
    -selector '"nomad_service" in value'

echo "writing binding-rule for Nomad tasks"
consul acl binding-rule create \
  -method 'nomad-workloads' \
  -description 'Binding rule for Nomad tasks authenticated using a workload identity' \
  -bind-type 'role' \
  -bind-name 'nomad-${value.nomad_namespace}-tasks' \
  -selector '"nomad_service" not in value'

echo "writing policy for Nomad tasks"
consul acl policy create -name policy-nomad-tasks -rules @${DIR}/consul-workload-identity/nomad-task-policy.hcl

echo "creating role for Nomad tasks using previously created policy"
consul acl role create -name nomad-default-tasks -policy-name policy-nomad-tasks

echo "Consul successfully configured to use Nomad Workload Identity!"

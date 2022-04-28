#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# unseal vault and get a root operator token; the vault is configured to
# autounseal with AWS KMS
while true :
do
    ROOT_TOKEN=$(vault operator init -recovery-shares=1 -recovery-threshold=1 | awk '/Initial Root Token/{print $4}')
    if [ ! -z $ROOT_TOKEN ]; then break; fi
    sleep 5
done
set -e

export VAULT_TOKEN="$ROOT_TOKEN"

mkdir -p ../keys
echo $VAULT_TOKEN > "${DIR}/../keys/vault_root_token"

# write policies for Nomad to Vault, and then configure Nomad to use the
# token from those policies

vault policy write nomad-server "${DIR}/vault-nomad-server-policy.hcl"
vault write /auth/token/roles/nomad-cluster "@${DIR}/vault-nomad-cluster-role.json"

NOMAD_VAULT_TOKEN=$(vault token create -policy nomad-server -period 72h -orphan | awk '/token /{print $2}')

cat <<EOF > "${DIR}/../keys/nomad_vault.hcl"
vault {
  enabled          = true
  address          = "http://active.vault.service.consul:8200"
  task_token_ttl   = "1h"
  create_from_role = "nomad-cluster"
  token            = "$NOMAD_VAULT_TOKEN"
}

EOF

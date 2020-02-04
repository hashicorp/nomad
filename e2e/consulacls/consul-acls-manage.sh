#!/usr/bin/env bash

# must be run from e2e directory

set -o errexit
set -o nounset
set -o pipefail

tfstatefile="terraform/terraform.tfstate"

# Make sure we are running from the e2e/ directory
[ "$(basename "$(pwd)")" == "e2e" ] || (echo "must be run from nomad/e2e directory" && exit 1)

# Make sure one argument was provided (subcommand)
[ ${#} -eq 1 ] || (echo "expect one argument (subcommand)" && exit 1)

# Make sure terraform state file exists
[ -f "${tfstatefile}" ] || (echo "file ${tfstatefile} must exist (run terraform?)" && exit 1)

# Load Linux Client Node IPs from terraform state file
linux_clients=$(jq -r .outputs.linux_clients.value[] <"${tfstatefile}" | xargs)

# Load Windows Client Node IPs from terraform state file
windows_clients=$(jq -r .outputs.windows_clients.value[] <"${tfstatefile}" | xargs)

# Combine all the clients together
# clients="${linux_clients} ${windows_clients}"

# Load Server Node IPs from terraform/terraform.tfstate
servers=$(jq -r .outputs.servers.value[] <"${tfstatefile}" | xargs)

# Use the 0th server as the ACL bootstrap server
server0=$(echo "${servers}" | cut -d' ' -f1)

# Find the .pem file to use
pemfile="terraform/$(jq -r '.resources[] | select(.name=="private_key_pem") | .instances[0].attributes.filename' <"terraform/terraform.tfstate")"

# See AWS service file
consul_configs="/etc/consul.d"
nomad_configs="/etc/nomad.d"

# Not really present in the config
user=ubuntu

# Create a filename based on the TF state file (.serial), where we will store and/or
# lookup the consul master token. The presense of this file is what determines
# whether a full ACL bootstrap must occur, or if we only need to activate ACLs
# whenever the "enable" sub-command is chosen.
token_file="/tmp/e2e-consul-bootstrap-$(jq .serial <${tfstatefile}).token"

# One argument - the subcommand to run which may be: bootstrap, enable, or disable
subcommand="${1}"

echo "==== SETUP configuration ====="
echo "SETUP command is: ${subcommand}"
echo "SETUP token file: ${token_file}"
echo "SETUP servers: ${servers}"
echo "SETUP linux clients: ${linux_clients}"
echo "SETUP windows clients: ${windows_clients}"
echo "SETUP pem file: ${pemfile}"
echo "SETUP consul configs: ${consul_configs}"
echo "SETUP nomad configs: ${nomad_configs}"
echo "SETUP aws user: ${user}"
echo "SETUP bootstrap server: ${server0}"

function doSSH() {
  hostname="$1"
  command="$2"
  echo "-----> will ssh command '${command}' on ${hostname}"
  ssh \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -i "${pemfile}" \
    "${user}@${hostname}" "${command}"
}

function doSCP() {
  original="$1"
  username="$2"
  hostname="$3"
  destination="$4"
  echo "------> will scp ${original} to ${hostname}"
  scp \
    -o StrictHostKeyChecking=no \
    -o UserKnownHostsFile=/dev/null \
    -i "${pemfile}" \
    "${original}" "${username}@${hostname}:${destination}"
}

function doBootstrap() {
  echo "=== Bootstrap: Consul Configs ==="

  # Stop all Nomad agents.
  stopNomad

  # Run the pre-activation step, which uploads an acl.hcl file (with default:allow)
  # to each Consul configuration directory, then (re)starts each
  # Consul agent.
  doPreActivateACLs

  echo "=== Bootstrap: Consul ACL Bootstrap ==="
  echo "sleeping 2 minutes to let Consul agents settle (avoid Legacy mode error)..."
  sleep 120

  # Bootstrap Consul ACLs on server[0]
  echo "-> bootstrap ACL using ${server0}"
  consul_http_token=$(doSSH "${server0}" "/usr/local/bin/consul acl bootstrap" | grep SecretID | awk '{print $2}')
  consul_http_addr="http://${server0}:8500"
  export CONSUL_HTTP_TOKEN=${consul_http_token}
  export CONSUL_HTTP_ADDR=${consul_http_addr}
  echo "  consul http: ${CONSUL_HTTP_ADDR}"
  echo "  consul root: ${CONSUL_HTTP_TOKEN}"
  echo "${CONSUL_HTTP_TOKEN}" > "${token_file}"

  # Create Consul Server Policy & Consul Server agent tokens
  echo "-> configure consul server policy"
  consul acl policy create -name server-policy -rules @consulacls/consul-server-policy.hcl

  # Create & Set agent token for each Consul Server
  for server in ${servers}; do
    echo "---> will create agent token for server ${server}"
    server_agent_token=$(consul acl token create -description "consul server agent token" -policy-name server-policy | grep SecretID | awk '{print $2}')
    echo "---> setting token for server agent: ${server} -> ${server_agent_token}"
    (export CONSUL_HTTP_ADDR="${server}:8500";  consul acl set-agent-token agent "${server_agent_token}")
    echo "---> done setting agent token for server ${server}"
  done

  # Wait 30s before continuing with configuring consul clients.
  echo "-> sleep 3s before continuing with clients"
  sleep 3

  # Create Consul Client Policy & Client agent tokens
  echo "-> configure consul client policy"
  consul acl policy create -name client-policy -rules @consulacls/consul-client-policy.hcl

  # Create & Set agent token for each Consul Client (excluding Windows)
  for linux_client in ${linux_clients}; do
    echo "---> will create consul agent token for client ${linux_client}"
    client_agent_token=$(consul acl token create -description "consul client agent token" -policy-name client-policy | grep SecretID | awk '{print $2}')
    echo "---> setting consul token for consul client ${linux_client} -> ${client_agent_token}"
    (export CONSUL_HTTP_ADDR="${linux_client}:8500"; consul acl set-agent-token agent "${client_agent_token}")
    echo "---> done setting agent token for client ${linux_client}"
  done

  # Now, upload the ACL policy file with default:deny so that ACL are actually
  # enforced.
  doActivateACLs

  echo "=== Bootstrap: Nomad Configs ==="

  # Create Nomad Server consul Policy and Nomad Server consul tokens
  echo "-> configure nomad server policy & consul token"
  consul acl policy create -name nomad-server-policy -rules @consulacls/nomad-server-policy.hcl
  nomad_server_consul_token=$(consul acl token create -description "nomad server consul token" -policy-name nomad-server-policy | grep SecretID | awk '{print $2}')
  nomad_server_consul_token_tmp=$(mktemp)
  cp consulacls/nomad-server-consul.hcl "${nomad_server_consul_token_tmp}"
  sed -i "s/CONSUL_TOKEN/${nomad_server_consul_token}/g" "${nomad_server_consul_token_tmp}"
  for server in ${servers}; do
    echo "---> upload nomad-server-consul.hcl to ${server}"
    doSCP "${nomad_server_consul_token_tmp}" "${user}" "${server}" "/tmp/nomad-server-consul.hcl"
    doSSH "${server}" "sudo mv /tmp/nomad-server-consul.hcl ${nomad_configs}/nomad-server-consul.hcl"
  done

  # Create Nomad Client consul Policy and Nomad Client consul token
  echo "-> configure nomad client policy & consul token"
  consul acl policy create -name nomad-client-policy -rules @consulacls/nomad-client-policy.hcl
  nomad_client_consul_token=$(consul acl token create -description "nomad client consul token" -policy-name nomad-client-policy | grep SecretID | awk '{print $2}')
  nomad_client_consul_token_tmp=$(mktemp)
  cp consulacls/nomad-client-consul.hcl "${nomad_client_consul_token_tmp}"
  sed -i "s/CONSUL_TOKEN/${nomad_client_consul_token}/g" "${nomad_client_consul_token_tmp}"
  for linux_client in ${linux_clients}; do
    echo "---> upload nomad-client-token.hcl to ${linux_client}"
    doSCP "${nomad_client_consul_token_tmp}" "${user}" "${linux_client}" "/tmp/nomad-client-consul.hcl"
    doSSH "${linux_client}" "sudo mv /tmp/nomad-client-consul.hcl ${nomad_configs}/nomad-client-consul.hcl"
  done

  startNomad

  export NOMAD_ADDR="http://${server0}:4646"

  echo "=== Activate: DONE ==="
}

function doSetAllowUnauthenticated {
  value="${1}"
  [ "${value}" == "true" ] || [ "${value}" == "false" ] || ( echo "allow_unauthenticated must be 'true' or 'false'" && exit 1)
  for server in ${servers}; do
    if [ "${value}" == "true" ]; then
      echo "---> setting consul.allow_unauthenticated=true on ${server}"
      doSSH "${server}" "sudo sed -i 's/allow_unauthenticated = false/allow_unauthenticated = true/g' ${nomad_configs}/nomad-server-consul.hcl"
    else
      echo "---> setting consul.allow_unauthenticated=false on ${server}"
      doSSH "${server}" "sudo sed -i 's/allow_unauthenticated = true/allow_unauthenticated = false/g' ${nomad_configs}/nomad-server-consul.hcl"
    fi
    doSSH "${server}" "sudo systemctl restart nomad"
  done

  for linux_client in ${linux_clients}; do
    if [ "${value}" == "true" ]; then
      echo "---> comment out consul token for Nomad client ${linux_client}"
      doSSH "${linux_client}" "sudo sed -i 's!token =!// token =!g' ${nomad_configs}/nomad-client-consul.hcl"
    else
      echo "---> un-comment consul token for Nomad client ${linux_client}"
      doSSH "${linux_client}" "sudo sed -i 's!// token =!token =!g' ${nomad_configs}/nomad-client-consul.hcl"
    fi
    doSSH "${linux_client}" "sudo systemctl restart nomad"
  done
}

function doEnable {
  if [ ! -f "${token_file}" ]; then
    echo "ENABLE: token file does not exist, doing a full ACL bootstrap"
    doBootstrap
  else
    echo "ENABLE: token file already exists, will activate ACLs"
    doSetAllowUnauthenticated "false"
    doActivateACLs
  fi

  echo "=== Enable: DONE ==="

  # show the status of all the agents
  echo "---> token file is ${token_file}"
  consul_http_token=$(cat "${token_file}")
  export CONSUL_HTTP_TOKEN="${consul_http_token}"
  echo "export CONSUL_HTTP_TOKEN=${CONSUL_HTTP_TOKEN}"
  doStatus
}

function doDisable {
  if [ ! -f "${token_file}" ]; then
    echo "DISABLE: token file does not exist, did bootstrap ever happen?"
    exit 1
  else
    echo "DISABLE: token file exists, will deactivate ACLs"
    doSetAllowUnauthenticated "true"
    doDeactivateACLs
  fi

  echo "=== Disable: DONE ==="

  # show the status of all the agents
  unset CONSUL_HTTP_TOKEN
  doStatus
}

function doPreActivateACLs {
  echo "=== PreActivate (set default:allow) ==="

  stopConsul

  # Upload acl-pre-enable.hcl to each Consul agent's configuration directory.
  for agent in ${servers} ${linux_clients}; do
    echo " pre-activate: upload acl-pre-enable.hcl to ${agent}::acl.hcl"
    doSCP "consulacls/acl-pre-enable.hcl" "${user}" "${agent}" "/tmp/acl.hcl"
    doSSH "${agent}" "sudo mv /tmp/acl.hcl ${consul_configs}/acl.hcl"
  done

  # Start each Consul agent to pickup the new config.
  for agent in ${servers} ${linux_clients}; do
    echo " pre-activate: start Consul agent on ${agent}"
    doSSH "${agent}" "sudo systemctl start consul"
  done

  echo "=== PreActivate: DONE ==="
}

function doActivateACLs {
  echo "=== Activate (set default:deny) ==="

  stopConsul

  # Upload acl-enable.hcl to each Consul agent's configuration directory.
  for agent in ${servers} ${linux_clients}; do
    echo " activate: upload acl-enable.hcl to ${agent}::acl.hcl"
    doSCP "consulacls/acl-enable.hcl" "${user}" "${agent}" "/tmp/acl.hcl"
    doSSH "${agent}" "sudo mv /tmp/acl.hcl ${consul_configs}/acl.hcl"
  done

  # Start each Consul agent to pickup the new config.
  for agent in ${servers} ${linux_clients}; do
    echo " activate: restart Consul agent on ${agent} ..."
    doSSH "${agent}" "sudo systemctl start consul"
  done

  echo "--> activate ACLs sleep for 2 minutes to let Consul figure things out"
  sleep 120
  echo "=== Activate: DONE ==="
}

function stopNomad {
  echo "=== Stop Nomad agents ==="
  # Stop every Nomad agent (clients and servers) in preperation for Consul ACL
  # bootstrapping.
  for server in ${servers}; do
    echo " stop Nomad Server on ${server}"
    doSSH "${server}" "sudo systemctl stop nomad"
    sleep 1
  done

  for linux_client in ${linux_clients}; do
    echo " stop Nomad Client on ${linux_client}"
    doSSH "${linux_client}" "sudo systemctl stop nomad"
    sleep 1
  done

  echo "... all nomad agents stopped"
}

function startNomad {
  echo "=== Start Nomad agents ==="
  # Start every Nomad agent (clients and servers) after having Consul ACL
  # bootstrapped and configurations set for Nomad.
  for server in ${servers}; do
    echo " start Nomad Server on ${server}"
    doSSH "${server}" "sudo systemctl start nomad"
    sleep 1
  done

  # give the servers a chance to settle
  sleep 10

  for linux_client in ${linux_clients}; do
    echo " start Nomad Client on ${linux_client}"
    doSSH "${linux_client}" "sudo systemctl start nomad"
    sleep 3
  done

  # give the clients a long time to settle
  sleep 30

  echo "... all nomad agents started"
}

function stopConsul {
  echo "=== Stop Consul agents ==="
  # Stop every Nonsul agent (clients and servers) in preperation for Consul ACL
  # bootstrapping.
  for server in ${servers}; do
    echo " stop Consul Server on ${server}"
    doSSH "${server}" "sudo systemctl stop consul"
    sleep 1
  done

  for linux_client in ${linux_clients}; do
    echo " stop Consul Client on ${linux_client}"
    doSSH "${linux_client}" "sudo systemctl stop consul"
    sleep 1
  done

  echo "... all consul agents stopped"
}

function startConsulClients {
    echo "=== Start Consul Clients ==="
    # Start Consul Clients
    for linux_client in ${linux_clients}; do
      echo " start Consul Client on ${linux_client}"
      doSSH "${linux_client}" "sudo systemctl start consul"
      sleep 2
    done

    sleep 5 # let them settle
    echo "... all consul clients started"
}

function doDeactivateACLs {
  echo "=== Deactivate ==="
  # Upload acl-disable.hcl to each Consul agent's configuration directory.
  for agent in ${servers} ${linux_clients}; do
    echo " deactivate: upload acl-disable.hcl to ${agent}::acl.hcl"
    doSCP "consulacls/acl-disable.hcl" "${user}" "${agent}" "/tmp/acl.hcl"
    doSSH "${agent}" "sudo mv /tmp/acl.hcl ${consul_configs}/acl.hcl"
  done

  # Restart each Consul agent to pickup the new config.
  for agent in ${servers} ${linux_clients}; do
    echo " deactivate: restart Consul on ${agent} ..."
    doSSH "${agent}" "sudo systemctl restart consul"
  done

  # Wait 120s before moving on, Consul / Nomad need time to settle down.
  echo " deactivate: sleep 2m ..."
  sleep 120
}

function doStatus {
  # assumes CONSUL_HTTP_TOKEN is set (or not)
  echo "consul members"
  consul members
  echo ""
  echo "nomad server members"
  nomad server members
  echo ""
  echo "nomad node status"
  nomad node status
  echo ""
}

# It's the entrypoint to our script!
case "${subcommand}" in
  enable)
    doEnable
    ;;
  disable)
    doDisable
    ;;
  *)
    echo "incorrect subcommand ${subcommand}"
    exit 1
    ;;
esac

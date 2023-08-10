# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Note: the test environment must have the following values set:
# export HCP_CLIENT_ID=
# export HCP_CLIENT_SECRET=
# export CONSUL_HTTP_TOKEN=
# export CONSUL_HTTP_ADDR=

data "hcp_consul_cluster" "e2e_shared_consul" {
  cluster_id = var.hcp_consul_cluster_id
}

# policy and configuration for the Consul Agent

resource "consul_acl_policy" "consul_agent" {
  name        = "${local.random_name}_consul_agent_policy"
  datacenters = [var.hcp_consul_cluster_id]
  rules       = data.local_file.consul_policy_for_consul_agent.content
}

data "local_file" "consul_policy_for_consul_agent" {
  filename = "${path.root}/etc/acls/consul/consul-agent-policy.hcl"
}

resource "consul_acl_token" "consul_agent_token" {
  description = "Consul agent token"
  policies    = [consul_acl_policy.consul_agent.name]
  local       = true
}

data "consul_acl_token_secret_id" "consul_agent_token" {
  accessor_id = consul_acl_token.consul_agent_token.id
}

resource "local_sensitive_file" "consul_acl_file" {
  content = templatefile("etc/consul.d/client_acl.json", {
    token = data.consul_acl_token_secret_id.consul_agent_token.secret_id
  })
  filename        = "uploads/shared/consul.d/client_acl.json"
  file_permission = "0600"
}

resource "local_sensitive_file" "consul_ca_file" {
  content         = base64decode(data.hcp_consul_cluster.e2e_shared_consul.consul_ca_file)
  filename        = "uploads/shared/consul.d/ca.pem"
  file_permission = "0600"
}

resource "local_sensitive_file" "consul_config_file" {
  content         = base64decode(data.hcp_consul_cluster.e2e_shared_consul.consul_config_file)
  filename        = "uploads/shared/consul.d/consul_client.json"
  file_permission = "0644"
}

resource "local_sensitive_file" "consul_base_config_file" {
  content         = templatefile("${path.root}/etc/consul.d/clients.json", {})
  filename        = "uploads/shared/consul.d/consul_client_base.json"
  file_permission = "0644"
}

resource "local_sensitive_file" "consul_systemd_unit_file" {
  content         = templatefile("${path.root}/etc/consul.d/consul.service", {})
  filename        = "uploads/shared/consul.d/consul.service"
  file_permission = "0644"
}

# Nomad servers configuration for Consul

resource "consul_acl_policy" "nomad_servers" {
  name        = "${local.random_name}_nomad_server_policy"
  datacenters = [var.hcp_consul_cluster_id]
  rules       = data.local_file.consul_policy_for_nomad_server.content
}

data "local_file" "consul_policy_for_nomad_server" {
  filename = "${path.root}/etc/acls/consul/nomad-server-policy.hcl"
}

resource "consul_acl_token" "nomad_servers_token" {
  description = "Nomad servers token"
  policies    = [consul_acl_policy.nomad_servers.name]
  local       = true
}

data "consul_acl_token_secret_id" "nomad_servers_token" {
  accessor_id = consul_acl_token.nomad_servers_token.id
}

resource "local_sensitive_file" "nomad_server_config_for_consul" {
  content = templatefile("etc/nomad.d/consul.hcl", {
    token               = data.consul_acl_token_secret_id.nomad_servers_token.secret_id
    client_service_name = "client-${local.random_name}"
    server_service_name = "server-${local.random_name}"
  })
  filename        = "uploads/shared/nomad.d/server-consul.hcl"
  file_permission = "0600"
}

# Nomad clients configuration for Consul

resource "consul_acl_policy" "nomad_clients" {
  name        = "${local.random_name}_nomad_client_policy"
  datacenters = [var.hcp_consul_cluster_id]
  rules       = data.local_file.consul_policy_for_nomad_clients.content
}

data "local_file" "consul_policy_for_nomad_clients" {
  filename = "${path.root}/etc/acls/consul/nomad-client-policy.hcl"
}

resource "consul_acl_token" "nomad_clients_token" {
  description = "Nomad clients token"
  policies    = [consul_acl_policy.nomad_clients.name]
  local       = true
}

data "consul_acl_token_secret_id" "nomad_clients_token" {
  accessor_id = consul_acl_token.nomad_clients_token.id
}

resource "local_sensitive_file" "nomad_client_config_for_consul" {
  content = templatefile("etc/nomad.d/consul.hcl", {
    token               = data.consul_acl_token_secret_id.nomad_clients_token.secret_id
    client_service_name = "client-${local.random_name}"
    server_service_name = "server-${local.random_name}"
  })
  filename        = "uploads/shared/nomad.d/client-consul.hcl"
  file_permission = "0600"
}

# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Consul token for bootstrapping the Consul server
resource "random_uuid" "consul_initial_management_token" {}

# Write to token to disk for developer convenience
resource "local_sensitive_file" "consul_initial_management_token" {
  content         = random_uuid.consul_initial_management_token.result
  filename        = "keys/consul_initial_management_token"
  file_permission = "0600"
}

# Verify Consul is serving HTTP, which we can use to gate all Consul API requests
data "http" "consul_ready" {
  url         = "https://${aws_instance.consul_server.public_ip}:8501"
  ca_cert_pem = tls_self_signed_cert.ca.cert_pem
  retry {
    attempts     = 300
    min_delay_ms = 1000
  }
}

# resource "terraform_data" "consul_bootstrap" {
#   provisioner "local-exec" {
#     command = "consul acl bootstrap -token=file=${path.root}/keys/consul_initial_management_token"
#     environment = {
#       CONSUL_HTTP_ADDR = data.http.consul_ready.url
#       CONSUL_CACERT    = local_sensitive_file.ca_cert.filename
#     }
#   }
# }

provider "consul" {
  address = data.http.consul_ready.url
  ca_pem  = data.local_sensitive_file.ca_cert.content
  token   = random_uuid.consul_initial_management_token.result
}


# Consul namespaces for test setup
resource "consul_namespace" "prod" {
  name        = "prod"
  description = "Production namespace (E2E testing only)"
}

resource "consul_namespace" "dev" {
  name        = "dev"
  description = "Development namespace (E2E testing only)"
}

# Consul policy and tokens for the Consul agents

resource "random_uuid" "consul_agent_token" {}

resource "local_sensitive_file" "consul_agent_token" {
  content         = random_uuid.consul_agent_token.result
  filename        = "keys/consul_agent_token"
  file_permission = "0600"
}

resource "consul_acl_policy" "consul_agents" {
  name  = "consul-agents"
  rules = file("etc/acls/consul/consul-agent-policy.hcl")
}

# the consul_acl_token resource doesn't let us set the secret ID
# so we need to set it via the CLI
resource "terraform_data" "consul_agent_token" {
  provisioner "local-exec" {
    command = "consul acl token create -policy-name=${consul_acl_policy.consul_agents.name} -token=file=${local_sensitive_file.consul_agent_token.filename}"
    environment = {
      CONSUL_HTTP_ADDR  = data.http.consul_ready.url
      CONSUL_CACERT = local_sensitive_file.ca_cert.filename
      CONSUL_HTTP_TOKEN = random_uuid.consul_initial_management_token.result
    }
  }
}

# Consul policy and tokens for the Nomad agents

resource "random_uuid" "consul_token_for_nomad" {}

resource "local_sensitive_file" "consul_token_for_nomad" {
  content         = random_uuid.consul_token_for_nomad.result
  filename        = "keys/consul_token_for_nomad"
  file_permission = "0600"
}

resource "consul_acl_policy" "nomad_cluster" {
  name  = "nomad-cluster"
  rules = file("etc/acls/consul/nomad-cluster-consul-policy.hcl")
}

# the consul_acl_token resource doesn't let us set the secret ID
# so we need to set it via the CLI
resource "terraform_data" "consul_token_for_nomad" {
  provisioner "local-exec" {
    command = "consul acl token create -policy-name=${consul_acl_policy.nomad_cluster.name} -token=file=${local_sensitive_file.consul_token_for_nomad.filename}"
    environment = {
      CONSUL_HTTP_ADDR  = data.http.consul_ready.url
      CONSUL_CACERT = local_sensitive_file.ca_cert.filename
      CONSUL_HTTP_TOKEN = random_uuid.consul_initial_management_token.result
    }
  }
}

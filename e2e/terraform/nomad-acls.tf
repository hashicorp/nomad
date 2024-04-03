# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

resource "random_uuid" "nomad_bootstrap_token" {}

# Write to token to disk for developer convenience
resource "local_sensitive_file" "nomad_token" {
  content         = random_uuid.nomad_bootstrap_token.result
  filename        = "${path.root}/keys/nomad_root_token"
  file_permission = "0600"
}

# Verify Nomad is serving HTTP, which we can use to gate all Nomad API requests
data "http" "nomad_ready" {
  url         = "https://${aws_instance.server.0.public_ip}:4646"
  ca_cert_pem = tls_self_signed_cert.ca.cert_pem
  retry {
    attempts     = 300
    min_delay_ms = 1000
  }
}

resource "terraform_data" "nomad_bootstrap" {
  provisioner "local-exec" {
    command = "${path.root}/etc/acls/nomad/bootstrap.sh ${path.root}/keys/nomad_root_token"
    environment = {
      NOMAD_ADDR   = data.http.nomad_ready.url
      NOMAD_CACERT = local_sensitive_file.ca_cert.filename
    }
  }
}

provider "nomad" {
  address   = data.http.nomad_ready.url
  ca_pem    = data.local_sensitive_file.ca_cert.content
  cert_pem  = data.local_sensitive_file.api_client_cert.content
  key_pem   = data.local_sensitive_file.api_client_key.content
  secret_id = random_uuid.nomad_bootstrap_token.result
}

# Anon users get agent:read only
resource "nomad_acl_policy" "anon" {
  depends_on  = [terraform_data.nomad_bootstrap]
  name        = "anonymous"
  description = "Anonymous policy"
  rules_hcl   = file("etc/acls/nomad/anonymous.nomad_policy.hcl")
}

# push the token out to the servers for humans to use.
# cert/key files are placed by ./provision-nomad module.
# this is here instead of there, because the servers
# must be provisioned before the token can be made,
# so this avoids a dependency cycle.
locals {
  root_nomad_env = <<EXEC
cat <<ENV | sudo tee -a /root/.bashrc
export NOMAD_ADDR=https://localhost:4646
export NOMAD_SKIP_VERIFY=true
export NOMAD_CLIENT_CERT=/etc/nomad.d/tls/agent.crt
export NOMAD_CLIENT_KEY=/etc/nomad.d/tls/agent.key
export NOMAD_TOKEN=${random_uuid.nomad_bootstrap_token.result}
export CONSUL_HTTP_ADDR=https://localhost:8501
export CONSUL_HTTP_TOKEN="${random_uuid.consul_initial_management_token.result}"
export CONSUL_CACERT=/etc/consul.d/ca.pem
ENV
EXEC
}

resource "null_resource" "root_nomad_env_servers" {
  count = var.server_count
  connection {
    type        = "ssh"
    user        = "ubuntu"
    host        = aws_instance.server[count.index].public_ip
    port        = 22
    private_key = file("${path.root}/keys/${local.random_name}.pem")
    timeout     = "5m"
  }
  provisioner "remote-exec" {
    inline = [local.root_nomad_env]
  }
}

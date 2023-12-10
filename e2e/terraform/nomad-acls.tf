# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Bootstrapping Nomad ACLs:
# We can't both bootstrap the ACLs and use the Nomad TF provider's
# resource.nomad_acl_token in the same Terraform run, because there's no way
# to get the management token into the provider's environment after we bootstrap.
# So we run a bootstrapping script and write our management token into a file
# that we read in for the output of $(terraform output environment) later.

locals {
  nomad_env = "NOMAD_ADDR=https://${aws_instance.server.0.public_ip}:4646 NOMAD_CACERT=keys/tls_ca.crt NOMAD_CLIENT_CERT=keys/tls_api_client.crt NOMAD_CLIENT_KEY=keys/tls_api_client.key"
}

resource "null_resource" "bootstrap_nomad_acls" {
  depends_on = [module.nomad_server]
  triggers = {
    script = data.template_file.bootstrap_nomad_script.rendered
  }

  provisioner "local-exec" {
    command = data.template_file.bootstrap_nomad_script.rendered
  }
}

# write the bootstrap token to the keys/ directory (where the ssh key is)
# so that we can read it into the data.local_file later. If not set,
# ensure that it's empty.
data "template_file" "bootstrap_nomad_script" {
  template = "${local.nomad_env} ./scripts/bootstrap-nomad.sh"
}

data "local_sensitive_file" "nomad_token" {
  depends_on = [null_resource.bootstrap_nomad_acls]
  filename   = "${path.root}/keys/nomad_root_token"
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
export NOMAD_TOKEN=${data.local_sensitive_file.nomad_token.content}
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

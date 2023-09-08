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

data "local_file" "nomad_token" {
  depends_on = [null_resource.bootstrap_nomad_acls]
  filename   = "${path.root}/keys/nomad_root_token"
}

locals {

  vault_env = var.tls ? "VAULT_ADDR=https://${aws_instance.server.0.public_ip}:8200 VAULT_CACERT=keys/tls_ca.crt VAULT_CLIENT_CERT=keys/tls_api_client.crt VAULT_CLIENT_KEY=keys/tls_api_client.key" : "VAULT_ADDR=http://${aws_instance.server.0.public_ip}:8200"
}

resource "null_resource" "bootstrap_vault" {
  depends_on = [
    aws_instance.server,
    module.nomad_server
  ]
  triggers = {
    script = data.template_file.bootstrap_vault_script.rendered
  }

  provisioner "local-exec" {
    command = data.template_file.bootstrap_vault_script.rendered
  }
}

# write the bootstrap token to the keys/ directory (where the ssh key is)
# so that we can read it into the data.local_file later. If not set,
# ensure that it's empty.
data "template_file" "bootstrap_vault_script" {
  template = var.vault ? "${local.vault_env} ./scripts/bootstrap-vault.sh" : "mkdir -p ${path.root}/keys; echo > ${path.root}/keys/vault_root_token; echo ${path.root}/keys/nomad_vault.hcl"
}

data "local_file" "vault_token" {
  depends_on = [null_resource.bootstrap_vault]
  filename   = "${path.root}/keys/vault_root_token"
}

data "local_file" "nomad_vault_config" {
  depends_on = [null_resource.bootstrap_vault]
  filename   = "${path.root}/keys/nomad_vault.hcl"
}

resource "null_resource" "nomad_vault_config" {

  depends_on = [
    aws_instance.server,
    null_resource.bootstrap_vault
  ]

  triggers = {
    data = data.local_file.nomad_vault_config.content
  }

  count = var.server_count

  provisioner "file" {
    source      = "${path.root}/keys/nomad_vault.hcl"
    destination = "./nomad_vault.hcl"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo mv ./nomad_vault.hcl /etc/nomad.d/nomad_vault.hcl",
      "sudo systemctl restart nomad"
    ]
  }

  connection {
    type        = "ssh"
    user        = "ubuntu"
    host        = aws_instance.server[count.index].public_ip
    port        = 22
    private_key = file("${path.root}/keys/${local.random_name}.pem")
  }
}

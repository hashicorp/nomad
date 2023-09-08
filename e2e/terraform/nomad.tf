# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

module "nomad_server" {
  source     = "./provision-nomad"
  depends_on = [aws_instance.server]
  count      = var.server_count

  platform = "linux"
  arch     = "linux_amd64"
  role     = "server"
  index    = count.index
  instance = aws_instance.server[count.index]

  nomad_local_binary = count.index < length(var.nomad_local_binary_server) ? var.nomad_local_binary_server[count.index] : var.nomad_local_binary

  nomad_license = var.nomad_license
  tls_ca_key    = tls_private_key.ca.private_key_pem
  tls_ca_cert   = tls_self_signed_cert.ca.cert_pem

  connection = {
    type        = "ssh"
    user        = "ubuntu"
    port        = 22
    private_key = "${path.root}/keys/${local.random_name}.pem"
  }
}

# TODO: split out the different Linux targets (ubuntu, centos, arm, etc.) when
# they're available
module "nomad_client_ubuntu_jammy_amd64" {
  source     = "./provision-nomad"
  depends_on = [aws_instance.client_ubuntu_jammy_amd64]
  count      = var.client_count_ubuntu_jammy_amd64

  platform = "linux"
  arch     = "linux_amd64"
  role     = "client"
  index    = count.index
  instance = aws_instance.client_ubuntu_jammy_amd64[count.index]

  nomad_local_binary = count.index < length(var.nomad_local_binary_client_ubuntu_jammy_amd64) ? var.nomad_local_binary_client_ubuntu_jammy_amd64[count.index] : var.nomad_local_binary

  tls_ca_key  = tls_private_key.ca.private_key_pem
  tls_ca_cert = tls_self_signed_cert.ca.cert_pem

  connection = {
    type        = "ssh"
    user        = "ubuntu"
    port        = 22
    private_key = "${path.root}/keys/${local.random_name}.pem"
  }
}


# TODO: split out the different Windows targets (2016, 2019) when they're
# available
module "nomad_client_windows_2016_amd64" {
  source     = "./provision-nomad"
  depends_on = [aws_instance.client_windows_2016_amd64]
  count      = var.client_count_windows_2016_amd64

  platform = "windows"
  arch     = "windows_amd64"
  role     = "client"
  index    = count.index
  instance = aws_instance.client_windows_2016_amd64[count.index]

  nomad_local_binary = count.index < length(var.nomad_local_binary_client_windows_2016_amd64) ? var.nomad_local_binary_client_windows_2016_amd64[count.index] : ""

  tls_ca_key  = tls_private_key.ca.private_key_pem
  tls_ca_cert = tls_self_signed_cert.ca.cert_pem

  connection = {
    type        = "ssh"
    user        = "Administrator"
    port        = 22
    private_key = "${path.root}/keys/${local.random_name}.pem"
  }
}

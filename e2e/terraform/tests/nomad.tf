locals {
  # fake connection to satisfy module requirements
  connection = {
    type        = "ssh"
    user        = "ubuntu"
    host        = "192.168.1.1"
    port        = 22
    private_key = "example"
  }
}

module "nomad_server" {

  source     = "../provision-nomad"
  count      = var.server_count
  platform   = "linux_amd64"
  profile    = var.profile
  connection = local.connection

  nomad_version = count.index < length(var.nomad_version_server) ? var.nomad_version_server[count.index] : var.nomad_version

  nomad_sha = count.index < length(var.nomad_sha_server) ? var.nomad_sha_server[count.index] : var.nomad_sha

  nomad_local_binary = count.index < length(var.nomad_local_binary_server) ? var.nomad_local_binary_server[count.index] : var.nomad_local_binary

  nomad_enterprise = var.nomad_enterprise
}

module "nomad_client_linux" {

  source     = "../provision-nomad"
  count      = var.client_count
  platform   = "linux_amd64"
  profile    = var.profile
  connection = local.connection

  nomad_version = count.index < length(var.nomad_version_client_linux) ? var.nomad_version_client_linux[count.index] : var.nomad_version

  nomad_sha = count.index < length(var.nomad_sha_client_linux) ? var.nomad_sha_client_linux[count.index] : var.nomad_sha

  nomad_local_binary = count.index < length(var.nomad_local_binary_client_linux) ? var.nomad_local_binary_client_linux[count.index] : var.nomad_local_binary

  nomad_enterprise = var.nomad_enterprise
}

module "nomad_client_windows" {

  source     = "../provision-nomad"
  count      = var.windows_client_count
  platform   = "windows_amd64"
  profile    = var.profile
  connection = local.connection

  nomad_version = count.index < length(var.nomad_version_client_windows) ? var.nomad_version_client_windows[count.index] : var.nomad_version

  nomad_sha = count.index < length(var.nomad_sha_client_windows) ? var.nomad_sha_client_windows[count.index] : var.nomad_sha

  nomad_local_binary = count.index < length(var.nomad_local_binary_client_windows) ? var.nomad_local_binary_client_windows[count.index] : var.nomad_local_binary

  nomad_enterprise = var.nomad_enterprise
}

module "nomad_server" {

  source     = "./provision-nomad"
  depends_on = [aws_instance.server]
  count      = var.server_count

  platform = "linux_amd64"
  profile  = var.profile
  role     = "server"
  index    = count.index

  # The specific version of Nomad deployed will default to whichever one of
  # nomad_sha, nomad_version, or nomad_local_binary is set, but if you want to
  # deploy multiple versions you can use the nomad_*_server variables to
  # provide a list of builds
  nomad_version = count.index < length(var.nomad_version_server) ? var.nomad_version_server[count.index] : var.nomad_version

  nomad_sha = count.index < length(var.nomad_sha_server) ? var.nomad_sha_server[count.index] : var.nomad_sha

  nomad_local_binary = count.index < length(var.nomad_local_binary_server) ? var.nomad_local_binary_server[count.index] : var.nomad_local_binary

  nomad_enterprise = var.nomad_enterprise

  connection = {
    type        = "ssh"
    user        = "ubuntu"
    host        = "${aws_instance.server[count.index].public_ip}"
    port        = 22
    private_key = "${path.root}/keys/${local.random_name}.pem"
  }
}

# TODO: split out the different Linux targets (ubuntu, centos, arm, etc.) when
# they're available
module "nomad_client_linux" {

  source     = "./provision-nomad"
  depends_on = [aws_instance.client_linux]
  count      = var.client_count

  platform = "linux_amd64"
  profile  = var.profile
  role     = "client-linux"
  index    = count.index

  # The specific version of Nomad deployed will default to whichever one of
  # nomad_sha, nomad_version, or nomad_local_binary is set, but if you want to
  # deploy multiple versions you can use the nomad_*_client_linux
  # variables to provide a list of builds
  nomad_version = count.index < length(var.nomad_version_client_linux) ? var.nomad_version_client_linux[count.index] : var.nomad_version

  nomad_sha = count.index < length(var.nomad_sha_client_linux) ? var.nomad_sha_client_linux[count.index] : var.nomad_sha

  nomad_local_binary = count.index < length(var.nomad_local_binary_client_linux) ? var.nomad_local_binary_client_linux[count.index] : var.nomad_local_binary

  nomad_enterprise = var.nomad_enterprise

  connection = {
    type        = "ssh"
    user        = "ubuntu"
    host        = "${aws_instance.client_linux[count.index].public_ip}"
    port        = 22
    private_key = "${path.root}/keys/${local.random_name}.pem"
  }
}

# TODO: split out the different Windows targets (2016, 2019) when they're
# available
module "nomad_client_windows" {

  source     = "./provision-nomad"
  depends_on = [aws_instance.client_windows]
  count      = var.windows_client_count

  platform = "windows_amd64"
  profile  = var.profile
  role     = "client-windows"
  index    = count.index

  # The specific version of Nomad deployed will default to whichever one of
  # nomad_sha, nomad_version, or nomad_local_binary is set, but if you want to
  # deploy multiple versions you can use the nomad_*_client_windows
  # variables to provide a list of builds
  nomad_version = count.index < length(var.nomad_version_client_windows) ? var.nomad_version_client_windows[count.index] : var.nomad_version

  nomad_sha = count.index < length(var.nomad_sha_client_windows) ? var.nomad_sha_client_windows[count.index] : var.nomad_sha

  # if nomad_local_binary is in use, you must pass a nomad_local_binary_client_windows!
  nomad_local_binary = count.index < length(var.nomad_local_binary_client_windows) ? var.nomad_local_binary_client_windows[count.index] : ""

  nomad_enterprise = var.nomad_enterprise

  connection = {
    type        = "ssh"
    user        = "Administrator"
    host        = "${aws_instance.client_windows[count.index].public_ip}"
    port        = 22
    private_key = "${path.root}/keys/${local.random_name}.pem"
  }
}

provider "aws" {
  region = var.region
}

module "provision-infra" {
  source = "./provision-infra"

  server_count = var.client_count_linux
  client_count_linux =  var.client_count_linux
  client_count_windows_2016_amd64 = var.client_count_windows_2016_amd64
  nomad_local_binary = var.nomad_local_binary
}
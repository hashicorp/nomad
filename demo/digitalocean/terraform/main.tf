variable "ssh_keys" {}

resource "atlas_artifact" "nomad-digitalocean" {
  name    = "hashicorp/nomad-demo"
  type    = "digitalocean.image"
  version = "latest"
}

module "servers" {
  source   = "./server"
  region   = "nyc3"
  image    = "${atlas_artifact.nomad-digitalocean.id}"
  ssh_keys = "${var.ssh_keys}"
}

module "clients-nyc3" {
  source   = "./client"
  region   = "nyc3"
  count    = 1
  image    = "${atlas_artifact.nomad-digitalocean.id}"
  servers  = "${module.servers.addrs}"
  ssh_keys = "${var.ssh_keys}"
}

/*
module "clients-ams2" {
  source   = "./client"
  region   = "ams2"
  count    = 1
  image    = "${atlas_artifact.nomad-digitalocean.id}"
  servers  = "${module.servers.addrs}"
  ssh_keys = "${var.ssh_keys}"
}

module "clients-ams3" {
  source   = "./client"
  region   = "ams3"
  count    = 1
  image    = "${atlas_artifact.nomad-digitalocean.id}"
  servers  = "${module.servers.addrs}"
  ssh_keys = "${var.ssh_keys}"
}

module "clients-sfo1" {
  source   = "./client"
  region   = "sfo1"
  count    = 1
  image    = "${atlas_artifact.nomad-digitalocean.id}"
  servers  = "${module.servers.addrs}"
  ssh_keys = "${var.ssh_keys}"
}
*/

output "cluster-info" {
  value = "Nomad Servers: ${join(" ", split(",", module.servers.addrs))}"
}

module "servers" {
  source = "./server"
  region = "nyc3"
  count  = 3
}

module "clients-ams2" {
  source  = "./client"
  region  = "ams2"
  count   = 500
  servers = "${module.servers.addrs}"
}

module "clients-ams3" {
  source  = "./client"
  region  = "ams3"
  count   = 500
  servers = "${module.servers.addrs}"
}

module "clients-nyc3" {
  source  = "./client"
  region  = "nyc3"
  count   = 500
  servers = "${module.servers.addrs}"
}

module "clients-sfo1" {
  source  = "./client"
  region  = "sfo1"
  count   = 500
  servers = "${module.servers.addrs}"
}

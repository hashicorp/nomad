backend "consul" {
  path = "vault/"
  address = "IP_ADDRESS:8500"
  cluster_addr = "https://IP_ADDRESS:8201"
  redirect_addr = "http://IP_ADDRESS:8200"
}

listener "tcp" {
  address = "IP_ADDRESS:8200"
  cluster_address = "IP_ADDRESS:8201"
  tls_disable = 1
}

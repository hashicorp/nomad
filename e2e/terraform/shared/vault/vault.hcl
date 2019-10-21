backend "consul" {
  path          = "vault/"
  address       = "{{ GetPrivateIP }}:8500"
  cluster_addr  = "https://{{ GetPrivateIP }}:8201"
  redirect_addr = "http://{{ GetPrivateIP }}:8200"
}

listener "tcp" {
  address         = "{{ GetPrivateIP }}:8200"
  cluster_address = "{{ GetPrivateIP }}:8201"
  tls_disable     = 1
}

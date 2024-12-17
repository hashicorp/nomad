# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

log_level      = "DEBUG"
data_dir       = "/opt/consul/data"
bind_addr      = "{{ GetPrivateIP }}"
advertise_addr = "{{ GetPrivateIP }}"
client_addr    = "0.0.0.0"

server = false

acl {
  enabled = true
  tokens {
    agent   = "${token}"
    default = "${token}"
  }
}

retry_join = ["provider=aws tag_key=ConsulAutoJoin tag_value=${autojoin_value}"]

tls {
  defaults {
    ca_file   = "/etc/consul.d/ca.pem"
    cert_file = "/etc/consul.d/cert.pem"
    key_file  = "/etc/consul.d/cert.key.pem"
  }
}

connect {
  enabled = true
}

service {
  name = "consul"
}

ports {
  grpc     = 8502
  grpc_tls = 8503
  dns      = 8600
}

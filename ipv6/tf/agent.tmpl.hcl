log_level = "debug"
data_dir = "/opt/nomad/data"

advertise {
  # https://developer.hashicorp.com/nomad/docs/configuration#advertise
  # https://pkg.go.dev/github.com/hashicorp/go-sockaddr/template
  http = "{{ GetPublicInterfaces | include \"type\" \"IPv6\" | limit 1 | attr \"address\" }}"
  rpc  = "{{ GetPublicInterfaces | include \"type\" \"IPv6\" | limit 1 | attr \"address\" }}"
  serf = "{{ GetPublicInterfaces | include \"type\" \"IPv6\" | limit 1 | attr \"address\" }}"
}

server {
  enabled          = true
  bootstrap_expect = ${count}
  server_join {
    # https://developer.hashicorp.com/nomad/docs/configuration/server_join
    # NOTE: these can be ipv6 with or without []
    retry_join = ::SERVER_IPS::
  }
}

client {
  enabled = true

  # https://developer.hashicorp.com/nomad/docs/configuration/client#servers
  # NOTE: ipv6 here needs [] around each addr.
  servers = ::SERVER_IPS::

  # use ipv6 for services
  preferred_address_family = "ipv6"
}


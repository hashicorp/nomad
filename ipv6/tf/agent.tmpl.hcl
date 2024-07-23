log_level = "debug"
data_dir = "/opt/nomad/data"

server {
  enabled          = true
  bootstrap_expect = ${count}
  server_join {
    # NOTE: these can be ipv6 with or without []
    retry_join = ::SERVER_IPS::
  }
}

client {
  enabled = true

  # NOTE: ipv6 here needs [] around each addr.
  servers = ::SERVER_IPS::

  #preferred_address_family = "ipv6"
}

plugin "docker" {
  config {
    allow_privileged = true
    gc {
      image = false
    }
  }
}

#plugin "nomad-driver-podman" {}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

ui {
  enabled = true
  label {
    text = "${name}"
  }
}


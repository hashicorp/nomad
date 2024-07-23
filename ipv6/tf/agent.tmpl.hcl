log_level = "debug"
data_dir = "/opt/nomad/data"

server {
  enabled          = true
  bootstrap_expect = ${count}
  #server_join {
  #  retry_join = []
  #}
}

client {
  enabled = true

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


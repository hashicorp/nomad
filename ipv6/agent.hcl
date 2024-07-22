log_level = "debug"
data_dir  = "/tmp/ipv6-data"
#plugin_dir = "/opt/nomad/plugins"

server {
  enabled          = true
  bootstrap_expect = 1
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
    volumes {
      enabled = true
    }
  }
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

#plugin "nomad-driver-podman" {}

ui {
  enabled = true
  label {
    text = "laptop ipv6"
  }
}


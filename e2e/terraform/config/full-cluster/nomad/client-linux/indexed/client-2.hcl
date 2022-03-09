datacenter = "dc2"

client {
  enabled = true

  meta {
    "rack" = "r1"
  }
}

plugin_dir = "/opt/nomad/plugins"
plugin "nomad-driver-podman" {
  config {
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

plugin "docker" {
  config {
    allow_privileged = true

    volumes {
      enabled = true
    }
  }
}

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}

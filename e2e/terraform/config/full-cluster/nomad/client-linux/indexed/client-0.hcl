# Enable the client
client {
  enabled = true

  meta {
    "rack" = "r1"
  }

  host_volume "shared_data" {
    path = "/srv/data"
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

plugin "nomad-driver-ecs" {
  config {
    enabled = true
    cluster = "nomad-rtd-e2e"
    region  = "us-east-1"
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

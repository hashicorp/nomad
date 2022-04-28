client {
  enabled = true

  options {
    "driver.raw_exec.enable"    = "1"
    "docker.privileged.enabled" = "true"
  }

  meta {
    "rack" = "r2"
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

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}

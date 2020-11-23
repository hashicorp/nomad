plugin_dir = "/opt/nomad/plugins"

client {
  enabled = true

  options {
    # Allow jobs to run as root
    "user.denylist" = ""

    # Allow rawexec jobs
    "driver.raw_exec.enable" = "1"

    # Allow privileged docker jobs
    "docker.privileged.enabled" = "true"
  }

  host_volume "shared_data" {
    path = "/srv/data"
  }
}

plugin "nomad-driver-podman" {
  config {
    volumes {
      enabled = true
    }
  }
}

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}

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

plugin "nomad-driver-ecs" {
  config {
    enabled = true
    cluster = "nomad-rtd-e2e"
    region  = "us-east-1"
  }
}

vault {
  enabled   = true
  address   = "https://active.vault.service.consul:8200"
  ca_file   = "/etc/vault.d/tls/ca.crt"
  cert_file = "/etc/vault.d/tls/agent.crt"
  key_file  = "/etc/vault.d/tls/agent.key"
}

# Enable the client
client {
  enabled = true

  options {
    "driver.raw_exec.enable"    = "1"
    "docker.privileged.enabled" = "true"
  }

  meta {
    "rack" = "r1"
  }

  host_volume "shared_data" {
    path = "/tmp/data"
  }
}

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}

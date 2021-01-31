datacenter = "dc2"

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

vault {
  enabled = true
  address = "http://active.vault.service.consul:8200"
}

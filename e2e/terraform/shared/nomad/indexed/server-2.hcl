server {
  enabled = true

  # this value will be overwritten during provisioning
  bootstrap_expect = 3 # SERVER_COUNT
}

vault {
  enabled          = false
  address          = "http://active.vault.service.consul:8200"
  task_token_ttl   = "1h"
  create_from_role = "nomad-cluster"
  token            = ""
}

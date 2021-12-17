client {
  enabled = true

  template {
    max_stale        = "300s"
    block_query_wait = "90s"

    wait {
      enabled = true
      min     = "2s"
      max     = "60s"
    }

    wait_bounds {
      enabled = true
      min     = "2s"
      max     = "60s"
    }

    consul_retry {
      enabled     = true
      attempts    = 5
      backoff     = "5s"
      max_backoff = "10s"
    }

    vault_retry {
      enabled     = true
      attempts    = 10
      backoff     = "15s"
      max_backoff = "20s"
    }
  }

}

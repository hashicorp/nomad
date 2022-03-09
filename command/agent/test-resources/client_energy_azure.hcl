client {
  enabled = true

  energy {
    provider = "azure"
    region   = "us-east-1"

    azure {
      client_id     = "client-id"
      client_secret = "client-secret"
      tenant_id     = "tenant-id"
    }
  }
}

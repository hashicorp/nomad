client {
  enabled = true

  energy {
    provider = "azure"
    region   = "us-east-1"

    azure {
      client_id = "client-id"
      tenant_id = "tenant-id"
    }
  }
}

client {
  enabled = true

  carbon {
    provider = "gcp"
    region = "us-east-1"

    gcp {
      service_account_key = ""
    }
  }
}

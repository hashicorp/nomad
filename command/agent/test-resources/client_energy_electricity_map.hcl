client {
  enabled = true

  energy {
    provider = "electricity-map"
    region   = "us-east-1"

    electricity_map {
      api_key = "key"
      api_url = "https://api.electricitymap.org/v3"
    }
  }
}

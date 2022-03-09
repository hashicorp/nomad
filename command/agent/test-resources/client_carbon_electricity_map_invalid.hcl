client {
  enabled = true

  carbon {
    provider = "electricity-map"
    region = "us-east-1"

    electricity_map {
      api_key = ""
      api_url = "https://api.electricitymap.org/v3"
    }
  }
}

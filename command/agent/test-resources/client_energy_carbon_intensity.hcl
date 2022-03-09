client {
  enabled = true

  energy {
    provider = "carbon-intensity"
    region   = "UK"

    carbon_intensity {
      api_url = "https://api.carbonintensity.org.uk/intensity"
    }
  }
}

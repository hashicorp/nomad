client {
  enabled = true

  energy {
    provider = "climatiq"
    region   = "us-east-1"

    carbon_intensity {
      cloud_provider "aws" {
        regions = ["us-east-1", "us-west-2"]
      }
      api_url = "https://beta3.api.climatiq.io/compute/{provider}/cpu"
    }
  }
}

plugin_dir = "/opt/nomad/plugins"

client {
  enabled = true
  options = {
    "user.denylist" = "www-data"
  }
}

plugin "nomad-driver-podman" {
  config {
    volumes {
      enabled = true
    }
  }
}

plugin "nomad-driver-ecs" {
  config {
    enabled = true
    cluster = "nomad-rtd-e2e"
    region  = "us-east-1"
  }
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

plugin "docker" {
  config {
    allow_privileged = true

    volumes {
      enabled = true
    }
  }
}

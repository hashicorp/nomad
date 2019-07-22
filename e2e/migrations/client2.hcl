log_level = "DEBUG"

data_dir = "/tmp/client2"

datacenter = "dc1"

client {
  enabled = true
  servers = ["127.0.0.1:4647"]

  meta {
    secondary = 0
  }
}

ports {
  http = 5657
}

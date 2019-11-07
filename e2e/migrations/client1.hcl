log_level = "DEBUG"

data_dir = "/tmp/client1"

datacenter = "dc1"

client {
  enabled = true
  servers = ["127.0.0.1:4647"]

  meta {
    secondary = 1
  }
}

ports {
  http = 5656
}

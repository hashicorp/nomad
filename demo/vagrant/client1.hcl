# Increase log verbosity
log_level = "DEBUG"

# Setup data dir
data_dir = "/tmp/client1"

name = "client1"

# Enable the client
client {
    enabled = true
    options {
        "driver.raw_exec.enable" = "1"
    }
}

advertise {
  http = "localhost"
  rpc = "localhost"
  serf = "localhost"
}

data_dir = "/tmp/nomad"
log_level = "DEBUG"
enable_debug = true
bind_addr = "0.0.0.0"
disable_update_check = true
client {
    enabled = true
    servers = ["nomad.service.consul:4647"]
    node_class = "linux-64bit"
}

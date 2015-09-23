data_dir = "/tmp/nomad"
log_level = "DEBUG"
enable_debug = true
bind_addr = "0.0.0.0"
disable_update_check = true
server {
    enabled = true
    bootstrap_expect = 1
}
client {
    enabled = true
    servers = ["127.0.0.1:4647"]
    node_class = "linux-64bit"
}

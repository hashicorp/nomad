log_file   = "C:\\opt\\nomad\\nomad.log"
plugin_dir = "C:\\opt\\nomad\\plugins"

client {
  enabled = true
}

plugin "raw_exec" {
  config {
    enabled = true
  }
}

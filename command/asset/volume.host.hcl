id        = "disk_prod_db1"
namespace = "default"
name      = "database"
type      = "host"
plugin_id = "plugin_id"

# Optional: for 'nomad volume create', specify a maximum and minimum capacity.
# Registering an existing volume will record but ignore these fields.
capacity_min = "10GiB"
capacity_max = "20G"

# Optional: for 'nomad volume create', specify one or more capabilities to
# validate. Registering an existing volume will record but ignore these fields.
# If omitted, the single-node-writer + file-system capability will be used as a
# default.
capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "block-device"
}

# Optional: provide a map of keys to string values expected by the plugin.
parameters {
  skuname = "Premium_LRS"
}

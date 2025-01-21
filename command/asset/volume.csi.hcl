id        = "ebs_prod_db1"
namespace = "default"
name      = "database"
type      = "csi"
plugin_id = "plugin_id"

# For 'nomad volume register', provide the external ID from the storage
# provider. This field should be omitted when creating a volume with
# 'nomad volume create'
external_id = "vol-23452345"

# For 'nomad volume create', specify a snapshot ID or volume to clone. You can
# specify only one of these two fields.
snapshot_id = "snap-12345"
# clone_id    = "vol-abcdef"

# Optional: for 'nomad volume create', specify a maximum and minimum capacity.
# Registering an existing volume will record but ignore these fields.
capacity_min = "10GiB"
capacity_max = "20G"

# Required (at least one): for 'nomad volume create', specify one or more
# capabilities to validate. Registering an existing volume will record but
# ignore these fields.
capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "block-device"
}

# Optional: for 'nomad volume create', specify mount options to validate for
# 'attachment_mode = "file-system". Registering an existing volume will record
# but ignore these fields.
mount_options {
  fs_type     = "ext4"
  mount_flags = ["ro"]
}

# Optional: specify one or more locations where the volume must be accessible
# from. Refer to the plugin documentation for what segment values are supported.
topology_request {
  preferred {
    topology { segments { rack = "R1" } }
  }
  required {
    topology { segments { rack = "R1" } }
    topology { segments { rack = "R2", zone = "us-east-1a" } }
  }
}

# Optional: provide any secrets specified by the plugin.
secrets {
  example_secret = "xyzzy"
}

# Optional: provide a map of keys to string values expected by the plugin.
parameters {
  skuname = "Premium_LRS"
}

# Optional: for 'nomad volume register', provide a map of keys to string
# values expected by the plugin. This field will populated automatically by
# 'nomad volume create'.
context {
  endpoint = "http://192.168.1.101:9425"
}

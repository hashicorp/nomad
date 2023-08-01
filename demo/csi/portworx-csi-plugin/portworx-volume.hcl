id           = "px-volume-1"
name         = "database"
type         = "csi"
plugin_id    = "portworx"
capacity_min = "1G"
capacity_max = "1G"

capability {
  access_mode     = "single-node-reader-only"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

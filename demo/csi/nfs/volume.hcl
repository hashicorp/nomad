id        = "my-nfs"
name      = "my-nfs"
type      = "csi"
plugin_id = "org.democratic-csi.nfs" # matches --csi-name in plugin tasks

capacity_min = "1MB"
capacity_max = "1GB"

capability {
  access_mode     = "multi-node-reader-only"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "multi-node-multi-writer"
  attachment_mode = "file-system"
}

mount_options {
  mount_flags = ["vers=4.2"] # specific to NFS version
}

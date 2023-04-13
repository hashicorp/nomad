id        = "csi-nfs"
name      = "csi-nfs"
type      = "csi"
plugin_id = "rocketduck-nfs"

capability {
  access_mode     = "multi-node-multi-writer"
  attachment_mode = "file-system"
}

type        = "csi"
id          = "efs-vol0"
name        = "efs-vol0"
external_id = "fs-07c2ad0ebc7c1e2ad"
plugin_id   = "aws-efs0"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader"
  attachment_mode = "file-system"
}

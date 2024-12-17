type        = "csi"
id          = "efs-vol0"
name        = "efs-vol0"
external_id = "fs-04c1d811ee34b324b"
plugin_id   = "aws-efs0"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-reader"
  attachment_mode = "file-system"
}

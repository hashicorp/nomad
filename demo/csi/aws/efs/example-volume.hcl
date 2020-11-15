# terraform output mytestefsvol >example-volume.hcl
# nomad volume register example-volume.hcl

type = "csi"
id = "mytestefsvol"
name = "my-test-efs-volume"
external_id = "fs-1a2b3c4d:/some/path"
access_mode = "multi-node-multi-writer"
attachment_mode = "file-system"
plugin_id = "aws-efs"

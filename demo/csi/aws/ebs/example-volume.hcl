# terraform output mytestebsvol >example-volume.hcl
# nomad volume register example-volume.hcl

type = "csi"
id = "mytestebsvol"
name = "my-test-ebs-volume"
external_id = "vol-04a08b15c16d23e42"
access_mode = "single-node-writer"
attachment_mode = "file-system"
plugin_id = "aws-ebs"

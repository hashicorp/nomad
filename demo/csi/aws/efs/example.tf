data "aws_kms_alias" "elasticfilesystem" {
  name = "alias/aws/elasticfilesystem"
}

resource "aws_efs_file_system" "mytestefsvol" {
  creation_token = "1eab7a71-2457-49b0-a56b-3519b21009a8"

  encrypted = true
  kms_key_id = data.aws_kms_alias.elasticfilesystem.target_key_arn

  #lifecycle_policy {
  #  transition_to_ia = AFTER_90_DAYS
  #}

  tags = {
    Name = "my-test-efs-volume"
  }
}

resource "aws_efs_mount_target" "mytestefsvol_mount" {
  file_system_id = aws_efs_file_system.mytestefsvol.id
  subnet_id      = aws_subnet.mysubnet.id
  ip_address     = ...
  security_groups = [
    ...
  ]
}

output "mytestefsvol" {
    value = <<__EOF__
# terraform output mytestefsvol >example-volume.hcl
# nomad volume register example-volume.hcl

type = "csi"
id = "mytestefsvol"
name = "my-test-efs-volume"
external_id = "${aws_ebs_volume.mytestefsvol.id}:/some/path"
access_mode = "multi-node-multi-writer"
attachment_mode = "file-system"
plugin_id = "aws-efs"
__EOF__
}

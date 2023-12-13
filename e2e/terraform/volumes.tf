# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

resource "aws_efs_file_system" "csi" {
  count          = var.volumes ? 1 : 0
  creation_token = "${local.random_name}-CSI"

  tags = {
    Name = "${local.random_name}-efs"
    User = data.aws_caller_identity.current.arn
  }
}

resource "aws_efs_mount_target" "csi" {
  count           = var.volumes ? 1 : 0
  file_system_id  = aws_efs_file_system.csi[0].id
  subnet_id       = data.aws_subnet.default.id
  security_groups = [aws_security_group.nfs[0].id]
}

data "template_file" "efs_volume_hcl" {
  count    = var.volumes ? 1 : 0
  template = <<EOT
type = "csi"
id = "efs-vol0"
name = "efs-vol0"
external_id = "${aws_efs_file_system.csi[0].id}"
plugin_id = "aws-efs0"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode = "single-node-reader"
  attachment_mode = "file-system"
}

EOT
}

resource "local_file" "efs_volume_hcl" {
  count           = var.volumes ? 1 : 0
  content         = data.template_file.efs_volume_hcl[0].rendered
  filename        = "${path.module}/../csi/input/volume-efs.hcl"
  file_permission = "0664"
}

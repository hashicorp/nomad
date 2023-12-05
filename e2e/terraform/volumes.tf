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

resource "local_file" "efs_volume_hcl" {
  count = var.volumes ? 1 : 0
  content = templatefile("${path.module}/volumes.tftpl", {
    id = aws_efs_file_system.csi[0].id,
  })
  filename        = "${path.module}/../csi/input/volume-efs.hcl"
  file_permission = "0664"
}

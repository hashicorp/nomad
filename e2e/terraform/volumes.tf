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

resource "aws_ebs_volume" "csi" {
  count             = var.volumes ? 1 : 0
  availability_zone = var.availability_zone
  size              = 40

  tags = {
    Name = "${local.random_name}-ebs"
    User = data.aws_caller_identity.current.arn
  }
}

data "template_file" "ebs_volume_hcl" {
  count    = var.volumes ? 1 : 0
  template = <<EOT
type = "csi"
id = "ebs-vol[0]"
name = "ebs-vol"
external_id = "${aws_ebs_volume.csi[0].id}"
access_mode = "single-node-writer"
attachment_mode = "file-system"
plugin_id = "aws-ebs0"
EOT
}

data "template_file" "efs_volume_hcl" {
  count    = var.volumes ? 1 : 0
  template = <<EOT
type = "csi"
id = "efs-vol0"
name = "efs-vol0"
external_id = "${aws_efs_file_system.csi[0].id}"
access_mode = "single-node-writer"
attachment_mode = "file-system"
plugin_id = "aws-efs0"
EOT
}

resource "local_file" "ebs_volume_hcl" {
  count           = var.volumes ? 1 : 0
  content         = data.template_file.ebs_volume_hcl[0].rendered
  filename        = "${path.module}/../csi/input/volume-ebs.hcl"
  file_permission = "0664"
}

resource "local_file" "efs_volume_hcl" {
  count           = var.volumes ? 1 : 0
  content         = data.template_file.efs_volume_hcl[0].rendered
  filename        = "${path.module}/../csi/input/volume-efs.hcl"
  file_permission = "0664"
}

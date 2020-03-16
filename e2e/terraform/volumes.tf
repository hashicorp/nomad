resource "aws_efs_file_system" "csi" {
  creation_token = "${local.random_name}-CSI"

  tags = {
    Name = "${local.random_name}-efs"
    User = data.aws_caller_identity.current.arn
  }
}

resource "aws_efs_mount_target" "csi" {
  file_system_id  = aws_efs_file_system.csi.id
  subnet_id       = data.aws_subnet.default.id
  security_groups = [aws_security_group.nfs.id]
}

resource "aws_ebs_volume" "csi" {
  availability_zone = var.availability_zone
  size              = 40

  tags = {
    Name = "${local.random_name}-ebs"
    User = data.aws_caller_identity.current.arn
  }
}

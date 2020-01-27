resource "aws_instance" "server" {
  ami                    = data.aws_ami.linux.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.server_count

  # Instance tags
  tags = {
    Name           = "${local.random_name}-server-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }

  iam_instance_profile = aws_iam_instance_profile.instance_profile.name
}

resource "aws_instance" "client_linux" {
  ami                    = data.aws_ami.linux.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.client_count
  depends_on             = [aws_instance.server]

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }

  ebs_block_device {
    device_name           = "/dev/xvdd"
    volume_type           = "gp2"
    volume_size           = "50"
    delete_on_termination = "true"
  }

  iam_instance_profile = aws_iam_instance_profile.instance_profile.name
}

resource "aws_instance" "client_windows" {
  ami                    = data.aws_ami.windows.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.windows_client_count
  depends_on             = [aws_instance.server]
  iam_instance_profile   = aws_iam_instance_profile.instance_profile.name

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-windows-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }

  ebs_block_device {
    device_name           = "xvdd"
    volume_type           = "gp2"
    volume_size           = "50"
    delete_on_termination = "true"
  }

  # We need this userdata script because Windows machines don't
  # configure ssh with cloud-init by default.
  user_data = file("${path.root}/shared/config/userdata-windows.ps1")

}

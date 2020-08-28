resource "aws_instance" "server" {
  ami                    = data.aws_ami.linux.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.server_count
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  user_data = file("${path.root}/userdata/ubuntu-bionic.sh")

  # Instance tags
  tags = {
    Name           = "${local.random_name}-server-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }
}

resource "aws_instance" "client_linux" {
  ami                    = data.aws_ami.linux.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.client_count
  depends_on             = [aws_instance.server]
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  user_data = file("${path.root}/userdata/ubuntu-bionic.sh")

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
}

resource "aws_instance" "client_windows" {
  ami                    = data.aws_ami.windows.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.windows_client_count
  depends_on             = [aws_instance.server]
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  user_data = file("${path.root}/userdata/windows-2016.ps1")

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
}

data "aws_ami" "linux" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["nomad-e2e-*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Ubuntu"]
  }
}

data "aws_ami" "windows" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["nomad-e2e-windows-2016*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Windows2016"]
  }
}

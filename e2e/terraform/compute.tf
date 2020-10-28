resource "aws_instance" "server" {
  ami                    = data.aws_ami.ubuntu_bionic_amd64.image_id
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

resource "aws_instance" "client_ubuntu_bionic_amd64" {
  ami                    = data.aws_ami.ubuntu_bionic_amd64.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.client_count_ubuntu_bionic_amd64
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  user_data = file("${path.root}/userdata/ubuntu-bionic.sh")

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-ubuntu-bionic-amd64-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }
}

resource "aws_instance" "client_windows_2016_amd64" {
  ami                    = data.aws_ami.windows_2016_amd64.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.client_count_windows_2016_amd64
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  user_data = file("${path.root}/userdata/windows-2016.ps1")

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-windows-2016-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }
}

data "aws_ami" "ubuntu_bionic_amd64" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["nomad-e2e-ubuntu-bionic-amd64-*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Ubuntu"]
  }
}

data "aws_ami" "windows_2016_amd64" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["nomad-e2e-windows-2016-amd64-*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Windows2016"]
  }
}

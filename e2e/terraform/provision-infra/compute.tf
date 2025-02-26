# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

locals {
  ami_prefix         = "nomad-e2e-v3"
  ubuntu_image_name  = "ubuntu-jammy-${var.instance_arch}"
  windows_image_name = "windows-2016-${var.instance_arch}"
}

resource "aws_instance" "server" {
  ami                    = data.aws_ami.ubuntu_jammy.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.servers.id] # see also the secondary ENI
  count                  = var.server_count
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  # Instance tags
  tags = {
    Name           = "${local.random_name}-server-${count.index}"
    ConsulAutoJoin = "auto-join-${local.random_name}"
    User           = data.aws_caller_identity.current.arn
  }
}

resource "aws_instance" "client_ubuntu_jammy" {
  ami                    = data.aws_ami.ubuntu_jammy.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.clients.id] # see also the secondary ENI
  count                  = var.client_count_linux
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-ubuntu-jammy-${count.index}"
    ConsulAutoJoin = "auto-join-${local.random_name}"
    User           = data.aws_caller_identity.current.arn
    OS             = "linux"
  }
}



resource "aws_instance" "client_windows_2016" {
  ami                    = data.aws_ami.windows_2016[0].image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.clients.id]
  count                  = var.client_count_windows_2016
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  user_data = file("${path.module}/userdata/windows-2016.ps1")

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-windows-2016-${count.index}"
    ConsulAutoJoin = "auto-join-${local.random_name}"
    User           = data.aws_caller_identity.current.arn
    OS             = "windows"
  }
}

resource "aws_instance" "consul_server" {
  ami                    = data.aws_ami.ubuntu_jammy_consul_server.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.consul_server.id]
  iam_instance_profile   = data.aws_iam_instance_profile.nomad_e2e_cluster.name
  availability_zone      = var.availability_zone

  # Instance tags
  tags = {
    Name           = "${local.random_name}-consul-server-ubuntu-jammy-amd64"
    ConsulAutoJoin = "auto-join-${local.random_name}"
    User           = data.aws_caller_identity.current.arn
  }
}


# We build the AMI only as needed. The AMI is tagged with the SHA of the commit
# that forced the build, which may not be the commit that's spawning this test
# run.
data "external" "packer_sha" {
  program = ["/bin/sh", "-c", <<EOT
set -e
REPO_ROOT=$(git rev-parse --show-toplevel)
sha=$(git log -n 1 --pretty=format:%H "$${REPO_ROOT}/e2e/terraform/packer")
echo "{\"sha\":\"$${sha}\"}"
EOT
  ]

}

# The Consul server's architecture isn't important for the E2E/Enos matrix, so
# we have a separate data source for that even though in practice it's currently
# always the same SHA as the linux_amd64 hosts for Nomad
data "aws_ami" "ubuntu_jammy_consul_server" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["${local.ami_prefix}-ubuntu-jammy-amd64-*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Ubuntu"]
  }

  filter {
    name   = "tag:BuilderSha"
    values = [data.external.packer_sha.result["sha"]]
  }
}

data "aws_ami" "ubuntu_jammy" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["${local.ami_prefix}-${local.ubuntu_image_name}-*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Ubuntu"]
  }

  filter {
    name   = "tag:BuilderSha"
    values = [data.external.packer_sha.result["sha"]]
  }
}

data "aws_ami" "windows_2016" {
  count = var.client_count_windows_2016 > 0 ? 1 : 0

  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["${local.ami_prefix}-${local.windows_image_name}-*"]
  }

  filter {
    name   = "tag:OS"
    values = ["Windows2016"]
  }

  filter {
    name   = "tag:BuilderSha"
    values = [data.external.packer_sha.result["sha"]]
  }
}

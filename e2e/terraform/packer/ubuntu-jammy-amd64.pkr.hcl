# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "build_sha" {
  type        = string
  description = "the revision of the packer scripts building this image"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
  distro    = "ubuntu-jammy-22.04-amd64-server-*"
  version   = "v3"
}

source "amazon-ebs" "latest_ubuntu_jammy" {
  ami_name             = "nomad-e2e-${local.version}-ubuntu-jammy-amd64-${local.timestamp}"
  iam_instance_profile = "packer_build" // defined in nomad-e2e repo
  instance_type        = "t3a.medium"
  region               = "us-east-1"
  ssh_username         = "ubuntu"
  ssh_interface        = "public_ip"

  source_ami_filter {
    filters = {
      architecture                       = "x86_64"
      "block-device-mapping.volume-type" = "gp2"
      name                               = "ubuntu/images/hvm-ssd/${local.distro}"
      root-device-type                   = "ebs"
      virtualization-type                = "hvm"
    }
    most_recent = true
    owners      = ["099720109477"] // Canonical
  }

  tags = {
    OS         = "Ubuntu"
    Version    = "Jammy"
    BuilderSha = var.build_sha
  }
}

build {
  sources = ["source.amazon-ebs.latest_ubuntu_jammy"]

  provisioner "file" {
    destination = "/tmp/linux"
    source      = "./ubuntu-jammy-amd64"
  }

  // cloud-init modifies the apt sources, so we need to wait
  // before running our setup
  provisioner "shell-local" {
    inline = ["sleep 30"]
  }

  provisioner "shell" {
    script = "./ubuntu-jammy-amd64/setup.sh"
  }
}

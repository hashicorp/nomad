# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

variable "build_sha" {
  type        = string
  description = "the revision of the packer scripts building this image"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
  version   = "v3"
}

source "amazon-ebs" "latest_ubuntu_jammy" {
  ami_name             = "nomad-e2e-${local.version}-ubuntu-jammy-amd64-${local.timestamp}"
  iam_instance_profile = "packer_build" // defined in nomad-e2e repo
  instance_type        = "m7a.large"
  region               = "us-east-1"
  ssh_username         = "ubuntu"
  ssh_interface        = "public_ip"

  # note: this is an internal baseline image and not available for use outside
  # of HashiCorp AWS environments. You'll need to use an Ubuntu base image from
  # Canonical if building outside that environment
  source_ami_filter {
    filters = {
      architecture = "x86_64"
      name         = "hc-base-ubuntu-2404-amd64-*"
      state        = "available"
    }
    most_recent = true
    owners      = ["888995627335"] # hc-ami_prod
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

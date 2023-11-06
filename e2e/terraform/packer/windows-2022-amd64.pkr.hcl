# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "build_sha" {
  type        = string
  description = "the revision of the packer scripts building this image"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
  version   = "v3"
}

source "amazon-ebs" "latest_windows_2022" {
  ami_name       = "nomad-e2e-${local.version}-windows-2022-amd64-${local.timestamp}"
  communicator   = "ssh"
  instance_type  = "m7a.xlarge"
  region         = "us-east-1"
  user_data_file = "windows-2022-amd64/userdata.ps1" # enables ssh
  ssh_timeout    = "10m"
  ssh_username   = "Administrator"

  source_ami_filter {
    filters = {
      name                = "Windows_Server-2022-English-Core-ECS_Optimized-*"
      root-device-type    = "ebs"
      virtualization-type = "hvm"
    }
    most_recent = true
    owners      = ["amazon"]
  }

  tags = {
    OS         = "Windows2022"
    BuilderSha = var.build_sha
  }
}

build {
  sources = ["source.amazon-ebs.latest_windows_2022"]

  provisioner "powershell" {
    scripts = [
      "windows-2022-amd64/disable-windows-updates.ps1",
      "windows-2022-amd64/install-consul.ps1",
      "windows-2022-amd64/install-nomad.ps1",
    ]
  }
}

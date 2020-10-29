locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
  distro    = "ubuntu-bionic-18.04-amd64-server-*"
}

source "amazon-ebs" "latest_ubuntu_bionic" {
  ami_name             = "nomad-e2e-ubuntu-bionic-amd64-${local.timestamp}"
  iam_instance_profile = "packer_build" // defined in nomad-e2e repo
  instance_type        = "t2.medium"
  region               = "us-east-1"
  ssh_username         = "ubuntu"

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
    OS      = "Ubuntu"
    Version = "Bionic"
  }
}

build {
  sources = ["source.amazon-ebs.latest_ubuntu_bionic"]

  provisioner "file" {
    destination = "/tmp/linux"
    source      = "./ubuntu-bionic-amd64"
  }

  provisioner "file" {
    destination = "/tmp/config"
    source      = "../config"
  }

  // cloud-init modifies the apt sources, so we need to wait
  // before running our setup
  provisioner "shell-local" {
    inline = ["sleep 30"]
  }

  provisioner "shell" {
    script = "./ubuntu-bionic-amd64/setup.sh"
  }
}

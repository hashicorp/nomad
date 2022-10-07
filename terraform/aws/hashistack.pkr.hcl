variable "hcp_bucket_name"{
  default="hashistack"
}

variable "image_name"{
  default="hashistack"
}

data "amazon-ami" "aws_base" {
  filters = {
    architecture                       = "x86_64"
    "block-device-mapping.volume-type" = "gp2"
    name                               = "ubuntu/images/hvm-ssd/ubuntu-xenial-16.04-amd64-server-*"
    root-device-type                   = "ebs"
    virtualization-type                = "hvm"
  }
  most_recent = true
  owners      = ["099720109477"]
  region      = "us-east-1"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "amazon-ebs" "aws_base" {
  ami_name      = "hashistack ${local.timestamp}"
  instance_type = "t2.small"
  region        = "us-east-1"
  source_ami    = "${data.amazon-ami.aws_base.id}"
  ssh_username  = "ubuntu"
}

build {
  hcp_packer_registry {
    bucket_name=var.hcp_bucket_name
    description = "This is our HashiStack image"
    bucket_labels = {
      "owner" = "Platform Team"
      "os" = "Ubuntu"
      "image-name" = var.image_name
    } 

    build_labels = {
      "build-time" = timestamp()
      "build-source" = basename (path.cwd)
    }
  } 


  sources = ["source.amazon-ebs.aws_base"]

  provisioner "shell" {
    inline = ["sudo mkdir /ops", "sudo chmod 777 /ops"]
  }

  provisioner "file" {
    destination = "/ops"
    source      = "../shared"
  }

  provisioner "file" {
    destination = "/ops"
    source      = "../examples"
  }

  provisioner "shell" {
    environment_vars = ["INSTALL_NVIDIA_DOCKER=true"]
    script           = "../shared/scripts/setup.sh"
  }

}

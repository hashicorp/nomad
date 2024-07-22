variable "name" {
  default = "ipv6-testing"
}

variable "pubkey" {
  default = "~/.ssh/linux-server.pub"
}

provider "aws" {
  region = "us-east-2"
  default_tags {
    tags = {
      Name = var.name
    }
  }
}

resource "aws_key_pair" "linux" {
  key_name   = "linux"
  public_key = file(pathexpand(var.pubkey))
}

# fedora doesn't ipv6 by default
#data "aws_ami" "fedora" {
#  owners = ["125523088429"]
#
#  most_recent = true
#
#  filter {
#    name   = "name"
#    values = ["Fedora-Cloud-Base-37-*"]
#  }
#
#  filter {
#    name   = "architecture"
#    values = ["x86_64"]
#  }
#}

resource "aws_instance" "mine" {
  #ami = data.aws_ami.fedora.id
  ami = "ami-0649bea3443ede307" # amazon linux

  instance_type = "t3.medium"
  key_name      = aws_key_pair.linux.key_name

  associate_public_ip_address = true

  subnet_id              = aws_subnet.mine.id
  vpc_security_group_ids = [aws_security_group.mine.id]

  ipv6_address_count = 1

  depends_on = [aws_internet_gateway.mine]
}

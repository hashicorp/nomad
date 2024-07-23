variable "name" {
  default = "ipv6-testing"
}

variable "instances" {
  default = 3
}

variable "nomad_path" {
  default = "../../pkg/linux_amd64/nomad"
}

provider "aws" {
  region = "us-east-2"
  default_tags {
    tags = {
      Name = var.name
    }
  }
}

module "keys" {
  name    = var.name
  path    = "${path.root}/keys"
  source  = "mitchellh/dynamic-keys/aws"
  version = "v2.0.0"
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
  count = var.instances

  #ami = data.aws_ami.fedora.id
  ami = "ami-0649bea3443ede307" # amazon linux

  instance_type = "t3.medium"
  key_name      = module.keys.key_name

  associate_public_ip_address = true

  subnet_id              = aws_subnet.mine.id
  vpc_security_group_ids = [aws_security_group.mine.id]

  ipv6_address_count = 1

  depends_on = [aws_internet_gateway.mine]

  # TODO: packer?
  # https://cloudinit.readthedocs.io/en/latest/reference/examples.html
  user_data = <<-EOF
    #cloud-config

    packages:
     - docker

    runcmd:
     - systemctl enable docker
     - systemctl start docker
     - systemctl enable nomad
     - mkdir -p /opt/cni/bin /opt/cni/config
     - curl -Lo /opt/cni/bin/cni.tgz https://github.com/containernetworking/plugins/releases/download/v1.5.1/cni-plugins-linux-amd64-v1.5.1.tgz
     - sh -c 'cd /opt/cni/bin/ && tar xzf cni.tgz && rm cni.tgz'

    write_files:
     - encoding: b64
       content: ${base64encode(local.agent_config)}
       path: /opt/nomad/agent.hcl
       owner: root
       permissions: '0644'
     - encoding: b64
       content: ${base64encode(local.systemd_unit)}
       path: /usr/lib/systemd/system/nomad.service
       owner: root
       permissions: '0644'
    EOF
}

locals {
  agent_config = templatefile("${path.module}/agent.tmpl.hcl",
    {
      count = var.instances,
      name  = var.name,
    }
  )
  systemd_unit = file("${path.module}/nomad.service")

  server_addrs = flatten(resource.aws_instance.mine.*.ipv6_addresses)
  encoded_addrs = jsonencode([
    for a in local.server_addrs : "[${a}]"
  ])
}

resource "null_resource" "files" {
  count = var.instances
  connection {
    type        = "ssh"
    user        = "ec2-user"
    port        = 22
    host        = local.server_addrs[count.index]
    private_key = module.keys.private_key_pem
    agent       = false # to avoid my SSH_ env vars
    timeout     = "5m"
  }
  provisioner "remote-exec" {
    inline = [
      "set -xe",
      "mkdir -p ~/bin/",
      "echo 'export PATH=$PATH:/home/ec2-user/bin' | sudo tee /etc/profile.d/nomad.sh",
      "sudo sed -i 's/::SERVER_IPS::/${local.encoded_addrs}/' /opt/nomad/agent.hcl",
    ]
  }
  provisioner "file" {
    source      = var.nomad_path
    destination = "/home/ec2-user/bin/nomad"
  }
  provisioner "remote-exec" {
    inline = [
      "set -xe",
      "chmod +x ~/bin/nomad",
      "sudo systemctl start nomad",
    ]
  }
}

# ansible inventory
#resource "local_file" "inventory" {
#  filename = "${path.module}/inventory"
#  content  = <<-EOF
#    all:
#      hosts:
#    %{for ip in aws_instance.mine.*.public_ip~}
#        ssh -i keys/${local.random_name}.pem ubuntu@${ip}
#    %{endfor~}
#    EOF
#}

variable "name" {
  description = "Used to name various infrastructure components"
  default     = "nomad-e2e"
}

variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}

variable "availability_zone" {
  description = "The AWS availability zone to deploy to."
  default     = "us-east-1a"
}

variable "indexed" {
  description = "Different configurations per client/server"
  default     = true
}

variable "instance_type" {
  description = "The AWS instance type to use for both clients and servers."
  default     = "t2.medium"
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

variable "windows_client_count" {
  description = "The number of windows clients to provision."
  default     = "1"
}

variable "nomad_sha" {
  description = "The sha of Nomad to write to provisioning output"
  default     = ""
}

provider "aws" {
  region = var.region
}

resource "random_pet" "e2e" {
}

resource "random_password" "windows_admin_password" {
  length           = 20
  special          = true
  override_special = "_%@"
}

locals {
  random_name = "${var.name}-${random_pet.e2e.id}"
}

# Generates keys to use for provisioning and access
module "keys" {
  name    = local.random_name
  path    = "${path.root}/keys"
  source  = "mitchellh/dynamic-keys/aws"
  version = "v2.0.0"
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

data "aws_caller_identity" "current" {
}

output "servers" {
  value = aws_instance.server.*.public_ip
}

output "linux_clients" {
  value = aws_instance.client_linux.*.public_ip
}

output "windows_clients" {
  value = aws_instance.client_windows.*.public_ip
}

output "message" {
  value = <<EOM
Your cluster has been provisioned! - To prepare your environment, run the
following:

```
export NOMAD_ADDR=http://${aws_instance.server[0].public_ip}:4646
export CONSUL_HTTP_ADDR=http://${aws_instance.server[0].public_ip}:8500
export NOMAD_E2E=1
```

Then you can run e2e tests with:

```
go test -v ./e2e
```

ssh into nodes with:
```
# server
ssh -i keys/${local.random_name}.pem ubuntu@${aws_instance.server[0].public_ip}

# clients
%{ for ip in aws_instance.client_linux.*.public_ip ~}
ssh -i keys/${local.random_name}.pem ubuntu@${ip}
%{ endfor ~}
```
EOM

}

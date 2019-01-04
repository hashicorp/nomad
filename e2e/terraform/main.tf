variable "name" {
  description = "Used to name various infrastructure components"
  default     = "nomad-e2e"
}

variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
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

variable "retry_join" {
  description = "Used by Consul to automatically form a cluster."
  default     = "provider=aws tag_key=ConsulAutoJoin tag_value=auto-join"
}

variable "nomad_sha" {
  description = "The sha of Nomad to run"
}

provider "aws" {
  region = "${var.region}"
}

resource "random_pet" "e2e" {}

locals {
  random_name = "${var.name}-${random_pet.e2e.id}"
}

# Generates keys to use for provisioning and access
module "keys" {
  name   = "nomad-e2e-${local.random_name}"
  path   = "${path.root}/keys"
  source = "mitchellh/dynamic-keys/aws"
}

data "aws_ami" "main" {
  most_recent = true
  owners      = ["self"]

  filter {
    name   = "name"
    values = ["nomad-e2e-*"]
  }
}

output "servers" {
  value = "${aws_instance.server.*.public_ip}"
}

output "clients" {
  value = "${aws_instance.client.*.public_ip}"
}

output "message" {
  value = <<EOM
Your cluster has been provisioned! - To prepare your environment, run the
following:

```
export NOMAD_ADDR=http://${aws_instance.client.0.public_ip}:4646
export CONSUL_HTTP_ADDR=http://${aws_instance.client.0.public_ip}:8500
export NOMAD_E2E=1
```

Then you can run e2e tests with:

```
go test -v ./e2e
```
EOM
}

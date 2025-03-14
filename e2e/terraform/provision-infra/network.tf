# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

data "aws_vpc" "default" {
  default = true
}

data "aws_subnet" "default" {
  availability_zone = var.availability_zone
  vpc_id            = data.aws_vpc.default.id
  default_for_az    = true
}

data "aws_subnet" "secondary" {
  availability_zone = var.availability_zone
  vpc_id            = data.aws_vpc.default.id
  default_for_az    = false
  tags = {
    Secondary = "true"
  }
}

# using a dns lookup instead of http, because it's faster
# and should be more reliable.
data "external" "my_public_ipv4" {
  program = ["/bin/sh", "-c", <<-EOT
    ip="$(dig @resolver4.opendns.com myip.opendns.com +short -4)"
    echo '{"ip": "'$ip'"}'
    EOT
  ]
}

locals {
  ingress_cidr = var.restrict_ingress_cidrblock ? "${chomp(data.external.my_public_ipv4.result["ip"])}/32" : "0.0.0.0/0"
}

resource "aws_security_group" "servers" {
  name   = "${local.random_name}-servers"
  vpc_id = data.aws_vpc.default.id

  # SSH from test runner
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # Nomad HTTP and RPC from test runner
  ingress {
    from_port   = 4646
    to_port     = 4647
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # Nomad HTTP access from the HashiCorp Cloud virtual network CIDR. This is
  # used for the workload identity authentication method JWKS callback.
  ingress {
    from_port   = 4646
    to_port     = 4646
    protocol    = "tcp"
    cidr_blocks = [var.hcp_hvn_cidr]
  }

  # Nomad HTTP and RPC from clients
  ingress {
    from_port       = 4646
    to_port         = 4647
    protocol        = "tcp"
    security_groups = [aws_security_group.clients.id]
  }

  # Nomad serf is covered here: only allowed between hosts in the servers own
  # security group so that clients can't accidentally use serf address
  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  # allow all outbound
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# Allows Consul network access to Nomad, which is necessary when creating
# auth methods. Must be done in an aws_security_group_rule to avoid cycles.
resource "aws_security_group_rule" "consul_ingress" {
  type                     = "ingress"
  from_port                = 0
  to_port                  = 0
  protocol                 = "-1"
  security_group_id        = aws_security_group.servers.id
  source_security_group_id = aws_security_group.consul_server.id
}

# the secondary VPC security group is intended only for internal traffic
# and so that we can exercise behaviors with multiple IPs
resource "aws_security_group" "servers_secondary" {
  name   = "${local.random_name}-servers-secondary"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  # allow all outbound
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "clients" {
  name   = "${local.random_name}-clients"
  vpc_id = data.aws_vpc.default.id

  # SSH from test runner
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # Nomad HTTP and RPC from test runner
  ingress {
    from_port   = 4646
    to_port     = 4647
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # UI reverse proxy from test runner
  ingress {
    from_port   = 6464
    to_port     = 6464
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # Fabio from test runner
  ingress {
    from_port   = 9998
    to_port     = 9999
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # allow all client-to-client
  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  # allow all outbound
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# the secondary VPC security group is intended only for internal traffic
# and so that we can exercise behaviors with multiple IPs
resource "aws_security_group" "clients_secondary" {
  name   = "${local.random_name}-clients-secondary"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  # allow all outbound
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "nfs" {
  count  = var.volumes ? 1 : 0
  name   = "${local.random_name}-nfs"
  vpc_id = data.aws_vpc.default.id

  ingress {
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = [aws_security_group.clients.id]
  }
}

# every server gets a ENI
resource "aws_network_interface" "servers_secondary" {
  subnet_id       = data.aws_subnet.secondary.id
  security_groups = [aws_security_group.servers_secondary.id]

  count = var.server_count
  attachment {
    instance     = aws_instance.server[count.index].id
    device_index = 1
  }
}

# every Linux client gets a ENI
resource "aws_network_interface" "clients_secondary" {
  subnet_id       = data.aws_subnet.secondary.id
  security_groups = [aws_security_group.clients_secondary.id]

  count = var.client_count_linux
  attachment {
    instance     = aws_instance.client_ubuntu_jammy[count.index].id
    device_index = 1
  }
}

resource "aws_security_group" "consul_server" {
  name   = "${local.random_name}-consul-server"
  vpc_id = data.aws_vpc.default.id

  # SSH from test runner
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # Consul HTTP and RPC from test runner
  ingress {
    from_port   = 8500
    to_port     = 8503
    protocol    = "tcp"
    cidr_blocks = [local.ingress_cidr]
  }

  # Consul HTTP and RPC from Consul agents
  ingress {
    from_port       = 8500
    to_port         = 8503
    protocol        = "tcp"
    security_groups = [aws_security_group.clients.id, aws_security_group.servers.id]
  }

  # Consul Server internal from Consul agents
  ingress {
    from_port       = 8300
    to_port         = 8302
    protocol        = "tcp"
    security_groups = [aws_security_group.clients.id, aws_security_group.servers.id]
  }

  # allow all outbound
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

variable "region" {}
variable "ami" {}
variable "instance_type" {}
variable "key_name" {}
variable "server_count" {}
variable "client_count" {}
variable "cluster_tag_value" {}

data "aws_vpc" "default" {
  default = true
}

resource "aws_security_group" "primary" {
  name   = "hashistack"
  vpc_id = "${data.aws_vpc.default.id}"

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Consul UI
  ingress {
    from_port   = 8500
    to_port     = 8500
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HDFS NameNode UI
  ingress {
    from_port   = 50070
    to_port     = 50070
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # HDFS DataNode UI
  ingress {
    from_port   = 50075
    to_port     = 50075
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Spark history server UI
  ingress {
    from_port   = 18080
    to_port     = 18080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    self      = true
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

data "template_file" "user_data_server_primary" {
  template = "${file("${path.root}/user-data-server.sh")}"

  vars {
    server_count      = "${var.server_count}"
    region            = "${var.region}"
    cluster_tag_value = "${var.cluster_tag_value}"
  }
}

data "template_file" "user_data_client" {
  template = "${file("${path.root}/user-data-client.sh")}"

  vars {
    region            = "${var.region}"
    cluster_tag_value = "${var.cluster_tag_value}"
  }
}

resource "aws_instance" "primary" {
  ami                    = "${var.ami}"
  instance_type          = "${var.instance_type}"
  key_name               = "${var.key_name}"
  vpc_security_group_ids = ["${aws_security_group.primary.id}"]
  count                  = "${var.server_count}"

  #Instance tags
  tags {
    Name           = "hashistack-server-${count.index}"
    ConsulAutoJoin = "${var.cluster_tag_value}"
  }

  user_data            = "${data.template_file.user_data_server_primary.rendered}"
  iam_instance_profile = "${aws_iam_instance_profile.instance_profile.name}"
}

resource "aws_instance" "client" {
  ami                    = "${var.ami}"
  instance_type          = "${var.instance_type}"
  key_name               = "${var.key_name}"
  vpc_security_group_ids = ["${aws_security_group.primary.id}"]
  count                  = "${var.client_count}"
  depends_on             = ["aws_instance.primary"]

  #Instance tags
  tags {
    Name           = "hashistack-client-${count.index}"
    ConsulAutoJoin = "${var.cluster_tag_value}"
  }

  user_data            = "${data.template_file.user_data_client.rendered}"
  iam_instance_profile = "${aws_iam_instance_profile.instance_profile.name}"
}

resource "aws_iam_instance_profile" "instance_profile" {
  name_prefix = "hashistack"
  role        = "${aws_iam_role.instance_role.name}"
}

resource "aws_iam_role" "instance_role" {
  name_prefix        = "hashistack"
  assume_role_policy = "${data.aws_iam_policy_document.instance_role.json}"
}

data "aws_iam_policy_document" "instance_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy" "auto_discover_cluster" {
  name   = "auto-discover-cluster"
  role   = "${aws_iam_role.instance_role.id}"
  policy = "${data.aws_iam_policy_document.auto_discover_cluster.json}"
}

data "aws_iam_policy_document" "auto_discover_cluster" {
  statement {
    effect = "Allow"

    actions = [
      "ec2:DescribeInstances",
      "ec2:DescribeTags",
      "autoscaling:DescribeAutoScalingGroups",
    ]

    resources = ["*"]
  }
}

output "primary_server_private_ips" {
  value = ["${aws_instance.primary.*.private_ip}"]
}

output "primary_server_public_ips" {
  value = ["${aws_instance.primary.*.public_ip}"]
}

output "client_private_ips" {
  value = ["${aws_instance.client.*.private_ip}"]
}

output "client_public_ips" {
  value = ["${aws_instance.client.*.public_ip}"]
}

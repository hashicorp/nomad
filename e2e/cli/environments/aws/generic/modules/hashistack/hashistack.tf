variable "name" {}
variable "region" {}
variable "ami" {}
variable "instance_type" {}
variable "server_count" {}
variable "client_count" {}
variable "retry_join" {}
variable "nomad_binary" {}

resource "aws_vpc" "nomad" {
  cidr_block = "10.0.0.0/16"
  enable_dns_hostnames = true
}

resource "aws_internet_gateway" "default" {
    vpc_id = "${aws_vpc.nomad.id}"
}

resource "aws_subnet" "us-east-1a-public" {
  vpc_id = "${aws_vpc.nomad.id}"

  cidr_block = "10.0.0.0/24"
  availability_zone = "us-east-1a"
  map_public_ip_on_launch = true
}

resource "aws_route_table" "us-east-1a-public" {
  vpc_id = "${aws_vpc.nomad.id}"

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = "${aws_internet_gateway.default.id}"
  }
}

resource "aws_route_table_association" "us-east-1a-public" {
    subnet_id = "${aws_subnet.us-east-1a-public.id}"
    route_table_id = "${aws_route_table.us-east-1a-public.id}"
}


resource "aws_security_group" "primary" {
  name_prefix   = "${var.name}"
  vpc_id = "${aws_vpc.nomad.id}"

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Nomad
  ingress {
    from_port   = 4646
    to_port     = 4646
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Consul
  ingress {
    from_port   = 8500
    to_port     = 8500
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # Vault
  ingress {
    from_port   = 8200
    to_port     = 8200
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

data "template_file" "user_data_server" {
  template = "${file("${path.root}/user-data-server.sh")}"

  vars {
    server_count = "${var.server_count}"
    region       = "${var.region}"
    retry_join   = "${var.retry_join}"
  }
}

data "template_file" "user_data_client" {
  template = "${file("${path.root}/user-data-client.sh")}"

  vars {
    region     = "${var.region}"
    retry_join = "${var.retry_join}"
  }
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

resource "aws_iam_role" "instance_role" {
  name_prefix        = "${var.name}"
  assume_role_policy = "${data.aws_iam_policy_document.instance_role.json}"
}

resource "aws_iam_instance_profile" "instance_profile" {
  name_prefix = "${var.name}"
  role        = "${aws_iam_role.instance_role.name}"
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

resource "aws_iam_role_policy" "auto_discover_cluster" {
  name   = "auto-discover-cluster"
  role   = "${aws_iam_role.instance_role.id}"
  policy = "${data.aws_iam_policy_document.auto_discover_cluster.json}"
}

resource "tls_private_key" "keypair" {
  algorithm = "RSA"
}

resource "aws_key_pair" "keypair" {
  key_name_prefix = "${var.name}"
  public_key = "${tls_private_key.keypair.public_key_openssh}"
}

resource "aws_instance" "server" {
  ami                    = "${var.ami}"
  instance_type          = "${var.instance_type}"
  key_name               = "${aws_key_pair.keypair.key_name}"
  vpc_security_group_ids = ["${aws_security_group.primary.id}"]
  subnet_id              = "${aws_subnet.us-east-1a-public.id}"
  count                  = "${var.server_count}"

  #Instance tags
  tags {
    Name           = "${var.name}-server-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  user_data            = "${data.template_file.user_data_server.rendered}"
  iam_instance_profile = "${aws_iam_instance_profile.instance_profile.name}"

  connection {
    type = "ssh"
    user = "ubuntu"
    private_key = "${tls_private_key.keypair.private_key_pem}"
  }

  provisioner "file" {
    source = "${var.nomad_binary}"
    destination = "/tmp/nomad"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo mv /tmp/nomad /usr/local/bin/nomad",
      "sudo chown root:root /usr/local/bin/nomad",
      "sudo chmod +x /usr/local/bin/nomad",
      "sudo systemctl start nomad"
    ]
  }
}

resource "aws_instance" "client" {
  ami                    = "${var.ami}"
  instance_type          = "${var.instance_type}"
  key_name               = "${aws_key_pair.keypair.key_name}"
  vpc_security_group_ids = ["${aws_security_group.primary.id}"]
  subnet_id              = "${aws_subnet.us-east-1a-public.id}"
  count                  = "${var.client_count}"
  depends_on             = ["aws_instance.server"]

  #Instance tags
  tags {
    Name           = "${var.name}-client-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  ebs_block_device =  {
    device_name                 = "/dev/xvdd"
    volume_type                 = "gp2"
    volume_size                 = "50"
    delete_on_termination       = "true"
  }

  user_data            = "${data.template_file.user_data_client.rendered}"
  iam_instance_profile = "${aws_iam_instance_profile.instance_profile.name}"

  connection {
    type = "ssh"
    user = "ubuntu"
    private_key = "${tls_private_key.keypair.private_key_pem}"
  }

  provisioner "file" {
    source = "${var.nomad_binary}"
    destination = "/tmp/nomad"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo mv /tmp/nomad /usr/local/bin/nomad",
      "sudo chown root:root /usr/local/bin/nomad",
      "sudo chmod +x /usr/local/bin/nomad",
      "sudo systemctl start nomad"
    ]
  }
}

resource "aws_elb" "server" {
  name = "nomad-e2e-servers"
  security_groups = ["${aws_security_group.primary.id}"]
  subnets = ["${aws_subnet.us-east-1a-public.id}"]
  health_check {
    healthy_threshold = 2
    unhealthy_threshold = 2
    timeout = 3
    interval = 30
    target = "TCP:4646"
  }

  listener {
    lb_port = 8500
    lb_protocol = "http"
    instance_port = "8500"
    instance_protocol = "http"
  }
  listener {
    lb_port = 8200
    lb_protocol = "http"
    instance_port = "8200"
    instance_protocol = "http"
  }
  listener {
    lb_port = 4646
    lb_protocol = "http"
    instance_port = "4646"
    instance_protocol = "http"
  }
}

resource "aws_elb_attachment" "server" {
  count = "${var.server_count}"
  elb = "${aws_elb.server.id}"
  instance = "${element(aws_instance.server.*.id, count.index)}"
}

output "server_dns" {
  value = "${aws_elb.server.dns_name}"
}


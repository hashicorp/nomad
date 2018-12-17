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
  count    = "${var.client_count}"

  vars {
    region     = "${var.region}"
    retry_join = "${var.retry_join}"
  }
}

data "template_file" "nomad_client_config" {
  template = "${file("${path.root}/configs/client.hcl")}"
}

data "template_file" "nomad_server_config" {
  template = "}"
}

resource "aws_instance" "server" {
  ami                    = "${data.aws_ami.main.image_id}"
  instance_type          = "${var.instance_type}"
  key_name               = "${module.keys.key_name}"
  vpc_security_group_ids = ["${aws_security_group.primary.id}"]
  count                  = "${var.server_count}"

  # Instance tags
  tags {
    Name           = "${local.random_name}-server-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  user_data            = "${data.template_file.user_data_server.rendered}"
  iam_instance_profile = "${aws_iam_instance_profile.instance_profile.name}"

  provisioner "file" {
    content     = "${file("${path.root}/configs/${var.indexed == false ? "server.hcl" : "indexed/server-${count.index}.hcl"}")}"
    destination = "/tmp/server.hcl"

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "remote-exec" {
    inline = [
      "aws s3 cp s3://nomad-team-test-binary/builds-oss/${var.nomad_sha}.tar.gz nomad.tar.gz",
      "sudo cp /ops/shared/config/nomad.service /etc/systemd/system/nomad.service",
      "sudo tar -zxvf nomad.tar.gz -C /usr/local/bin/",
      "sudo cp /tmp/server.hcl /etc/nomad.d/nomad.hcl",
      "sudo chmod 0755 /usr/local/bin/nomad",
      "sudo chown root:root /usr/local/bin/nomad",
      "sudo systemctl start nomad.service"
    ]

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }
}

resource "aws_instance" "client" {
  ami                    = "${data.aws_ami.main.image_id}"
  instance_type          = "${var.instance_type}"
  key_name               = "${module.keys.key_name}"
  vpc_security_group_ids = ["${aws_security_group.primary.id}"]
  count                  = "${var.client_count}"
  depends_on             = ["aws_instance.server"]

  # Instance tags
  tags {
    Name           = "${local.random_name}-client-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  ebs_block_device =  {
    device_name                 = "/dev/xvdd"
    volume_type                 = "gp2"
    volume_size                 = "50"
    delete_on_termination       = "true"
  }

  user_data            = "${element(data.template_file.user_data_client.*.rendered, count.index)}"
  iam_instance_profile = "${aws_iam_instance_profile.instance_profile.name}"

  provisioner "file" {
    content     = "${file("${path.root}/configs/${var.indexed == false ? "client.hcl" : "indexed/client-${count.index}.hcl"}")}"
    destination = "/tmp/client.hcl"

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "remote-exec" {
    inline = [
      "aws s3 cp s3://nomad-team-test-binary/builds-oss/${var.nomad_sha}.tar.gz nomad.tar.gz",
      "sudo tar -zxvf nomad.tar.gz -C /usr/local/bin/",
      "sudo cp /ops/shared/config/nomad.service /etc/systemd/system/nomad.service",
      "sudo cp /tmp/client.hcl /etc/nomad.d/nomad.hcl",
      "sudo chmod 0755 /usr/local/bin/nomad",
      "sudo chown root:root /usr/local/bin/nomad",
      "sudo systemctl start nomad.service"
    ]

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }
}


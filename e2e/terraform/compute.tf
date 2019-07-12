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

  provisioner "file" {
    content     = "${file("${path.root}/configs/${var.tls == false ? "notls.hcl" : "tls.hcl"}")}"
    destination = "/tmp/tls.hcl"

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }
  provisioner "local-exec" {
    command = <<EOF
openssl req -newkey rsa:2048 -nodes \
	-subj "/CN=server.global.nomad" \
	-keyout keys/agent-${self.public_ip}.key \
	-out keys/agent-${self.public_ip}.csr

cat <<'NEOY' > keys/agent-${self.public_ip}.conf
subjectAltName=DNS:server.global.nomad,DNS:localhost,IP:127.0.0.1,IP:${self.private_ip},IP:${self.public_ip}
extendedKeyUsage = serverAuth, clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
NEOY

openssl x509 -req -CAcreateserial \
	-extfile ./keys/agent-${self.public_ip}.conf \
	-days 365 \
  -sha256 \
	-CA keys/tls_ca.crt \
	-CAkey keys/tls_ca.key \
	-in keys/agent-${self.public_ip}.csr \
	-out keys/agent-${self.public_ip}.crt

EOF
  }

  provisioner "file" {
    content = "${file("${path.root}/keys/agent-${self.public_ip}.key")}"
    destination = "/tmp/agent.key"
    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "file" {
    content = "${file("${path.root}/keys/agent-${self.public_ip}.crt")}"
    destination = "/tmp/agent.crt"
    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "file" {
    content = "${tls_self_signed_cert.ca.cert_pem}"
    destination = "/tmp/ca.crt"
    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "remote-exec" {
    inline = [
      # basic installation
      "aws s3 cp s3://nomad-team-test-binary/builds-oss/${var.nomad_sha}.tar.gz nomad.tar.gz",
      "sudo cp /ops/shared/config/nomad.service /etc/systemd/system/nomad.service",
      "sudo tar -zxvf nomad.tar.gz -C /usr/local/bin/",
      "sudo chmod 0755 /usr/local/bin/nomad",
      "sudo chown root:root /usr/local/bin/nomad",

      # prepare config
      "sudo mkdir /etc/nomad.d/tls",
      "sudo cp /tmp/ca.crt /etc/nomad.d/tls/",
      "sudo cp /tmp/agent.crt /etc/nomad.d/tls/",
      "sudo cp /tmp/agent.key /etc/nomad.d/tls/",

      "sudo cp /tmp/server.hcl /etc/nomad.d/nomad.hcl",
      "sudo cp /tmp/tls.hcl /etc/nomad.d/tls.hcl",

      # run nomad
      "sudo systemctl enable nomad.service",
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

  provisioner "file" {
    content     = "${file("${path.root}/configs/${var.tls == false ? "notls.hcl" : "tls.hcl"}")}"
    destination = "/tmp/tls.hcl"

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }
  provisioner "local-exec" {
    command = <<EOF
openssl req -newkey rsa:2048 -nodes \
	-subj "/CN=client.global.nomad" \
	-keyout keys/agent-${self.public_ip}.key \
	-out keys/agent-${self.public_ip}.csr

cat <<'NEOY' > keys/agent-${self.public_ip}.conf
subjectAltName=DNS:server.global.nomad,DNS:localhost,IP:127.0.0.1,IP:${self.private_ip},IP:${self.public_ip}
extendedKeyUsage = serverAuth, clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
NEOY


openssl x509 -req -CAcreateserial \
	-extfile ./keys/agent-${self.public_ip}.conf \
	-days 365 \
  -sha256 \
	-CA keys/tls_ca.crt \
	-CAkey keys/tls_ca.key \
	-in keys/agent-${self.public_ip}.csr \
	-out keys/agent-${self.public_ip}.crt

EOF
  }

  provisioner "file" {
    content = "${file("${path.root}/keys/agent-${self.public_ip}.key")}"
    destination = "/tmp/agent.key"
    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "file" {
    content = "${file("${path.root}/keys/agent-${self.public_ip}.crt")}"
    destination = "/tmp/agent.crt"
    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "file" {
    content = "${tls_self_signed_cert.ca.cert_pem}"
    destination = "/tmp/ca.crt"
    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }

  provisioner "remote-exec" {
    inline = [
      # basic installation
      "aws s3 cp s3://nomad-team-test-binary/builds-oss/${var.nomad_sha}.tar.gz nomad.tar.gz",
      "sudo cp /ops/shared/config/nomad.service /etc/systemd/system/nomad.service",
      "sudo tar -zxvf nomad.tar.gz -C /usr/local/bin/",
      "sudo chmod 0755 /usr/local/bin/nomad",
      "sudo chown root:root /usr/local/bin/nomad",

      # prepare config
      "sudo mkdir /etc/nomad.d/tls",
      "sudo cp /tmp/ca.crt /etc/nomad.d/tls/",
      "sudo cp /tmp/agent.crt /etc/nomad.d/tls/",
      "sudo cp /tmp/agent.key /etc/nomad.d/tls/",

      "sudo cp /tmp/client.hcl /etc/nomad.d/nomad.hcl",
      "sudo cp /tmp/tls.hcl /etc/nomad.d/tls.hcl",

      # run nomad
      "sudo systemctl enable nomad.service",
      "sudo systemctl start nomad.service"
    ]

    connection {
      user        = "ubuntu"
      private_key = "${module.keys.private_key_pem}"
    }
  }
}


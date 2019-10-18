data "template_file" "user_data_server" {
  template = file("${path.root}/shared/user-data-server.sh")

  vars = {
    server_count = var.server_count
    region       = var.region
  }
}

data "template_file" "user_data_client" {
  template = file("${path.root}/shared/user-data-client.sh")
  count    = var.client_count

  vars = {
    region     = var.region
  }
}

resource "aws_instance" "server" {
  ami                    = data.aws_ami.main.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.server_count

  # Instance tags
  tags = {
    Name           = "${local.random_name}-server-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }

  user_data            = data.template_file.user_data_server.rendered
  iam_instance_profile = aws_iam_instance_profile.instance_profile.name

  # copy up all provisioning scripts and configs
  provisioner "file" {
    source      = "shared"
    destination = "/opt"

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }

  provisioner "remote-exec" {
    inline = [
      "/opt/shared/config/provision-server.sh ${var.nomad_sha} ${var.server_count}",
    ]

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }
}

resource "aws_instance" "client" {
  ami                    = data.aws_ami.main.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.client_count
  depends_on             = [aws_instance.server]

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-${count.index}"
    ConsulAutoJoin = "auto-join"
    SHA            = var.nomad_sha
    User           = data.aws_caller_identity.current.arn
  }

  ebs_block_device {
    device_name           = "/dev/xvdd"
    volume_type           = "gp2"
    volume_size           = "50"
    delete_on_termination = "true"
  }

  user_data            = element(data.template_file.user_data_client.*.rendered, count.index)
  iam_instance_profile = aws_iam_instance_profile.instance_profile.name

  # copy up all provisioning scripts and configs
  provisioner "file" {
    source      = "shared"
    destination = "/opt"

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }

  provisioner "remote-exec" {
    inline = [
      "/opt/shared/config/provision-client.sh ${var.nomad_sha} ${count.index}",
    ]

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }
}

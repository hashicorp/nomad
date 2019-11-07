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

  iam_instance_profile = aws_iam_instance_profile.instance_profile.name

  # copy up all provisioning scripts and configs
  provisioner "file" {
    source      = "shared/"
    destination = "/ops/shared"

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /ops/shared/config/provision-server.sh",
      "/ops/shared/config/provision-server.sh aws ${var.server_count} '${var.nomad_sha}' '${var.indexed == false ? "server.hcl" : "indexed/server-${count.index}.hcl"}'",
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

  iam_instance_profile = aws_iam_instance_profile.instance_profile.name

  # copy up all provisioning scripts and configs
  provisioner "file" {
    source      = "shared/"
    destination = "/ops/shared"

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /ops/shared/config/provision-client.sh",
      "/ops/shared/config/provision-client.sh aws '${var.nomad_sha}' '${var.indexed == false ? "client.hcl" : "indexed/client-${count.index}.hcl"}'",
    ]

    connection {
      host        = coalesce(self.public_ip, self.private_ip)
      type        = "ssh"
      user        = "ubuntu"
      private_key = module.keys.private_key_pem
    }
  }
}

resource "aws_instance" "server" {
  ami                    = data.aws_ami.linux.image_id
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
  ami                    = data.aws_ami.linux.image_id
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
data "template_file" "user_data_client_windows" {
  template = file("${path.root}/shared/config/userdata-windows.ps1")
  vars = {
    admin_password = random_password.windows_admin_password.result
  }
}

resource "aws_instance" "client_windows" {
  ami                    = data.aws_ami.windows.image_id
  instance_type          = var.instance_type
  key_name               = module.keys.key_name
  vpc_security_group_ids = [aws_security_group.primary.id]
  count                  = var.windows_client_count
  depends_on             = [aws_instance.server]
  iam_instance_profile   = "${aws_iam_instance_profile.instance_profile.name}"

  # Instance tags
  tags = {
    Name           = "${local.random_name}-client-windows-${count.index}"
    ConsulAutoJoin = "auto-join"
  }

  ebs_block_device {
    device_name           = "xvdd"
    volume_type           = "gp2"
    volume_size           = "50"
    delete_on_termination = "true"
  }

  # We need this userdata script because without it there's no
  # admin password for the winrm provisoner connection.
  user_data = data.template_file.user_data_client_windows.rendered

  # copy up all provisioning scripts and configs
  provisioner "file" {
    source      = "shared/"
    destination = "/ops/shared"

    connection {
      host     = coalesce(self.public_ip, self.private_ip)
      type     = "winrm"
      user     = "Administrator"
      password = "${random_password.windows_admin_password.result}"
    }
  }

  provisioner "remote-exec" {
    inline = [<<-EOS
/ops/shared/config/provision-windows-client.ps1 \
    --cloud aws \
    --sha ${var.nomad_sha} \
    --index ${count.index}
EOS
    ]

    connection {
      host     = coalesce(self.public_ip, self.private_ip)
      type     = "winrm"
      user     = "Administrator"
      password = "${random_password.windows_admin_password.result}"
    }
  }

}

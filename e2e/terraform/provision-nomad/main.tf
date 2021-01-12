locals {
  provision_script = var.platform == "windows_amd64" ? "C:/opt/provision.ps1" : "/opt/provision.sh"

  custom_path = dirname("${path.root}/config/custom/")

  custom_config_files = compact(setunion(
    fileset(local.custom_path, "nomad/*.hcl"),
    fileset(local.custom_path, "nomad/${var.role}/*.hcl"),
    fileset(local.custom_path, "nomad/${var.role}/indexed/*${var.index}.hcl"),
    fileset(local.custom_path, "consul/*.json"),
    fileset(local.custom_path, "consul/${var.role}/*.json"),
    fileset(local.custom_path, "consul${var.role}indexed/*${var.index}*.json"),
    fileset(local.custom_path, "vault/*.hcl"),
    fileset(local.custom_path, "vault${var.role}*.hcl"),
    fileset(local.custom_path, "vault${var.role}indexed/*${var.index}.hcl"),
  ))

  # abstract-away platform-specific parameter expectations
  _arg = var.platform == "windows_amd64" ? "-" : "--"

  tls_role = var.role == "server" ? "server" : "client"
}

resource "null_resource" "provision_nomad" {

  depends_on = [
    null_resource.upload_custom_configs,
    null_resource.upload_nomad_binary,
    null_resource.generate_instance_tls_certs
  ]

  # no need to re-run if nothing changes
  triggers = {
    script = data.template_file.provision_script.rendered
  }

  # Run the provisioner as a local-exec'd ssh command as a workaround for
  # Windows remote-exec zero-byte scripts bug:
  # https://github.com/hashicorp/terraform/issues/25634
  # https://github.com/hashicorp/terraform/blob/master/CHANGELOG.md#0150-unreleased
  #
  # The retry behavior and explicit PasswordAuthenticaiton flag here are to
  # workaround a race with the Windows userdata script that installs the
  # authorized_key. Unfortunately this still results in a bunch of "permission
  # denied" errors while waiting for those keys to be configured.
  provisioner "local-exec" {
    command = "set -x; until ssh -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no -o LogLevel=ERROR -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${var.connection.private_key} -p ${var.connection.port} ${var.connection.user}@${var.instance.public_ip} ${data.template_file.provision_script.rendered}; do sleep 5; done"
  }

}

data "template_file" "provision_script" {
  template = "${local.provision_script}${data.template_file.arg_nomad_sha.rendered}${data.template_file.arg_nomad_version.rendered}${data.template_file.arg_nomad_binary.rendered}${data.template_file.arg_nomad_enterprise.rendered}${data.template_file.arg_nomad_acls.rendered}${data.template_file.arg_nomad_tls.rendered}${data.template_file.arg_profile.rendered}${data.template_file.arg_role.rendered}${data.template_file.arg_index.rendered}"
}

data "template_file" "arg_nomad_sha" {
  template = var.nomad_sha != "" && var.nomad_local_binary == "" ? " ${local._arg}nomad_sha ${var.nomad_sha}" : ""
}

data "template_file" "arg_nomad_version" {
  template = var.nomad_version != "" && var.nomad_sha == "" && var.nomad_local_binary == "" ? " ${local._arg}nomad_version ${var.nomad_version}" : ""
}

data "template_file" "arg_nomad_binary" {
  template = var.nomad_local_binary != "" ? " ${local._arg}nomad_binary /tmp/nomad" : ""
}

data "template_file" "arg_nomad_enterprise" {
  template = var.nomad_enterprise ? " ${local._arg}enterprise" : ""
}

data "template_file" "arg_nomad_acls" {
  template = var.nomad_acls ? " ${local._arg}nomad_acls" : ""
}

data "template_file" "arg_nomad_tls" {
  template = var.tls ? " ${local._arg}tls" : ""
}

data "template_file" "arg_profile" {
  template = var.profile != "" ? " ${local._arg}config_profile ${var.profile}" : ""
}

data "template_file" "arg_role" {
  template = var.role != "" ? " ${local._arg}role ${var.role}" : ""
}

data "template_file" "arg_index" {
  template = var.index != "" ? " ${local._arg}index ${var.index}" : ""
}

resource "null_resource" "upload_nomad_binary" {

  count      = var.nomad_local_binary != "" ? 1 : 0
  depends_on = [null_resource.upload_custom_configs]
  triggers = {
    nomad_binary_sha = filemd5(var.nomad_local_binary)
  }

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.instance.public_ip
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "15m"
  }

  provisioner "file" {
    source      = var.nomad_local_binary
    destination = "/tmp/nomad"
  }
}

resource "null_resource" "upload_custom_configs" {

  count = var.profile == "custom" ? 1 : 0
  triggers = {
    hashes = join(",", [for file in local.custom_config_files : filemd5("${local.custom_path}/${file}")])
  }

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.instance.public_ip
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "15m"
  }

  provisioner "file" {
    source      = local.custom_path
    destination = "/tmp/"
  }
}

resource "null_resource" "generate_instance_tls_certs" {
  count = var.tls ? 1 : 0
  depends_on = [null_resource.upload_custom_configs]

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.instance.public_ip
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "15m"
  }

  provisioner "local-exec" {
    command = <<EOF
set -e

cat <<'EOT' > keys/ca.crt
${var.tls_ca_cert}
EOT

cat <<'EOT' > keys/ca.key
${var.tls_ca_key}
EOT

openssl req -newkey rsa:2048 -nodes \
	-subj "/CN=server.global.nomad" \
	-keyout keys/agent-${var.instance.public_ip}.key \
	-out keys/agent-${var.instance.public_ip}.csr

cat <<'NEOY' > keys/agent-${var.instance.public_ip}.conf

subjectAltName=DNS:server.global.nomad,DNS:server.dc1.consul,DNS:localhost,DNS:var.instance.public_dns,IP:127.0.0.1,IP:${var.instance.private_ip},IP:${var.instance.public_ip}
extendedKeyUsage = serverAuth, clientAuth
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
NEOY
openssl x509 -req -CAcreateserial \
	-extfile ./keys/agent-${var.instance.public_ip}.conf \
	-days 365 \
  -sha256 \
	-CA keys/ca.crt \
	-CAkey keys/ca.key \
	-in keys/agent-${var.instance.public_ip}.csr \
	-out keys/agent-${var.instance.public_ip}.crt

EOF
  }

  provisioner "remote-exec" {
    inline = [
      "mkdir -p /tmp/nomad-tls",
    ]
  }
  provisioner "file"{
    source = "keys/ca.crt"
    destination = "/tmp/nomad-tls/ca.crt"
  }
  provisioner "file" {
    source      = "keys/agent-${var.instance.public_ip}.crt"
    destination = "/tmp/nomad-tls/agent.crt"
  }
  provisioner "file" {
    source      = "keys/agent-${var.instance.public_ip}.key"
    destination = "/tmp/nomad-tls/agent.key"
  }
  # workaround to avoid updating packer
  provisioner "file" {
    source = "packer/ubuntu-bionic-amd64/provision.sh"
    destination = "/opt/provision.sh"
  }
  provisioner "file" {
    source = "config"
    destination = "/tmp/config"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo cp -r /tmp/nomad-tls /opt/config/${var.profile}/nomad/tls",
      "sudo cp -r /tmp/nomad-tls /opt/config/${var.profile}/consul/tls",
      "sudo cp -r /tmp/nomad-tls /opt/config/${var.profile}/vault/tls",

      # more workaround
      "sudo rm -rf /opt/config",
      "sudo mv /tmp/config /opt/config"
    ]
  }

}

locals {
  provision_script = var.platform == "windows_amd64" ? "C:/opt/provision.ps1" : "/opt/provision.sh"

  config_path = dirname("${path.root}/config/")

  config_files = compact(setunion(
    fileset(local.config_path, "**"),
  ))

  update_config_command = var.platform == "windows_amd64" ? "if (test-path /opt/config) { Remove-Item -Path /opt/config -Force -Recurse }; cp -r /tmp/config /opt/config" : "sudo rm -rf /opt/config; sudo mv /tmp/config /opt/config"

  # abstract-away platform-specific parameter expectations
  _arg = var.platform == "windows_amd64" ? "-" : "--"
}

resource "null_resource" "provision_nomad" {

  depends_on = [
    null_resource.upload_configs,
    null_resource.upload_nomad_binary
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
    command = "until ssh -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no -o LogLevel=ERROR -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${var.connection.private_key} -p ${var.connection.port} ${var.connection.user}@${var.connection.host} ${data.template_file.provision_script.rendered}; do sleep 5; done"
  }

}

data "template_file" "provision_script" {
  template = "${local.provision_script}${data.template_file.arg_nomad_sha.rendered}${data.template_file.arg_nomad_version.rendered}${data.template_file.arg_nomad_binary.rendered}${data.template_file.arg_nomad_enterprise.rendered}${data.template_file.arg_nomad_license.rendered}${data.template_file.arg_nomad_acls.rendered}${data.template_file.arg_profile.rendered}${data.template_file.arg_role.rendered}${data.template_file.arg_index.rendered}${data.template_file.autojoin_tag.rendered}"
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

data "template_file" "arg_nomad_license" {
  template = var.nomad_license != "" ? " ${local._arg}nomad_license ${var.nomad_license}" : ""
}

data "template_file" "arg_nomad_acls" {
  template = var.nomad_acls ? " ${local._arg}nomad_acls" : ""
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

data "template_file" "autojoin_tag" {
  template = var.cluster_name != "" ? " ${local._arg}autojoin auto-join-${var.cluster_name}" : ""
}

resource "null_resource" "upload_nomad_binary" {

  count      = var.nomad_local_binary != "" ? 1 : 0
  depends_on = [null_resource.upload_configs]
  triggers = {
    nomad_binary_sha = filemd5(var.nomad_local_binary)
  }

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.connection.host
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "15m"
  }

  provisioner "file" {
    source      = var.nomad_local_binary
    destination = "/tmp/nomad"
  }
}

resource "null_resource" "upload_configs" {

  triggers = {
    hashes = join(",", [for file in local.config_files : filemd5("${local.config_path}/${file}")])
  }

  connection {
    type        = "ssh"
    user        = var.connection.user
    host        = var.connection.host
    port        = var.connection.port
    private_key = file(var.connection.private_key)
    timeout     = "15m"
  }

  provisioner "file" {
    source      = local.config_path
    destination = "/tmp/"
  }

  provisioner "local-exec" {
    command = "until ssh -o PasswordAuthentication=no -o KbdInteractiveAuthentication=no -o LogLevel=ERROR -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -i ${var.connection.private_key} -p ${var.connection.port} ${var.connection.user}@${var.connection.host} '${local.update_config_command}'; do sleep 5; done"
  }

}

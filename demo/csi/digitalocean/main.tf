# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# Terraform configuration for creating a volume in DigitalOcean and
# registering it with Nomad

# create the volume
resource "digitalocean_volume" "test_volume" {
  region                  = var.region
  name                    = "csi-test-volume"
  size                    = 50
  initial_filesystem_type = "ext4"
  description             = "a volume for testing Nomad CSI"
}

# run the plugin job
resource "nomad_job" "plugin" {
  jobspec = templatefile("${path.module}/plugin.nomad", { token = var.do_token })

  hcl2 {
    enabled = true
  }
}

# register the volume with Nomad
resource "nomad_volume" "test_volume" {
  volume_id             = var.volume_id
  name                  = var.volume_id
  type                  = "csi"
  plugin_id             = "digitalocean"
  external_id           = digitalocean_volume.test_volume.id
  deregister_on_destroy = true

  capability {
    access_mode     = "single-node-writer"
    attachment_mode = "block-device"
  }
}

# consume the volume
resource "nomad_job" "redis" {
  jobspec    = templatefile("${path.module}/volume-job.nomad", { volume_id = nomad_volume.test_volume.id })
  depends_on = [nomad_volume.test_volume]

  hcl2 {
    enabled = true
  }
}

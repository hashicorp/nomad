# Terraform configuration for creating a volume in Hetzner Cloud and
# registering it with Nomad

# create the volume
resource "hcloud_volume" "test_volume" {
  location = var.location
  name     = "csi-test-volume"
  size     = 50
}

# run the plugin job
resource "nomad_job" "plugin" {
  jobspec = templatefile("${path.module}/plugin.nomad", {
    token                     = var.hcloud_token
    hcloud_csi_driver_version = var.hcloud_csi_driver_version
  })
}

# register the volume with Nomad
resource "nomad_volume" "test_volume" {
  volume_id             = var.volume_id
  name                  = var.volume_id
  type                  = "csi"
  plugin_id             = "hetzner"
  external_id           = hcloud_volume.test_volume.id
  access_mode           = "single-node-writer"
  attachment_mode       = "file-system"
  deregister_on_destroy = true
  mount_options {
    fs_type = "ext4"
  }
}

# consume the volume
resource "nomad_job" "redis" {
  jobspec    = templatefile("${path.module}/volume-job.nomad", { volume_id = nomad_volume.test_volume.id })
  depends_on = [nomad_volume.test_volume]
}

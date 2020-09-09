variable "name" {
    type        = string
    default     = "hashistack"
    description = "The default name to use for resources."
}

variable "project" {
  type        = string
  description = "The Google Cloud Platform project to deploy the Nomad cluster in."
}

variable "region" {
    type        = string
    default     = "us-east1"
    description = "The GCP region to deploy resources in."
}

variable "zone" {
  type        = string
  default     = "c"
  description = "The GCP zone to deploy resources in."
}

variable "csi_disk_count" {
  type        = number
  description = "The number of block devices to configure for CSI use."
  default     = "2"
}

variable "csi_disk_size_gb" {
  type        = number
  description = "The size in GB of the block devices to configure for CSI use."
  default     = "10"
}

variable "csi_disk_type" {
  type        = string
  description = "The size in GB of the block devices to configure for CSI use."
  default     = "pd-ssd"
}

data "google_compute_default_service_account" "default" {
    project = var.project
}

locals {
  shouldCreate = var.csi_disk_count > 0 ? 1 : 0
}

resource "google_project_iam_custom_role" "nomad" {
  count = local.shouldCreate
  role_id     = "nomad"
  title       = "Nomad CSI"
  description = "A description"
  permissions = [
    "compute.disks.get", 
    "compute.disks.use",
    "compute.instances.get", 
    "compute.instances.attachDisk", 
    "compute.instances.detachDisk"
  ]
}

resource "google_service_account" "nomad" {
  count = local.shouldCreate
  account_id   = "nomad-sa"
  display_name = "Nomad CSI Account"
}

resource "google_service_account_iam_member" "nomad-sa-csi" {
  count = local.shouldCreate
  service_account_id = google_service_account.nomad[count.index].name
  role               = google_project_iam_custom_role.nomad[count.index].id
  member             = "serviceAccount:${google_service_account.nomad[count.index].email}"
}

resource "google_project_iam_member" "nomad-sa-csi" {
  count = local.shouldCreate
  role               = google_project_iam_custom_role.nomad[count.index].id
  member             = "serviceAccount:${google_service_account.nomad[count.index].email}"
}

# Allow SA service account use the default GCE account
resource "google_service_account_iam_member" "gce-default-account-iam" {
  count = local.shouldCreate
  service_account_id = data.google_compute_default_service_account.default.name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.nomad[count.index].email}"
}

resource "google_service_account_key" "nomad-sa-key" {
  count = local.shouldCreate
  service_account_id = google_service_account.nomad[count.index].id
  public_key_type    = "TYPE_X509_PEM_FILE"
}

resource "local_file" "nomad-sa-key-file" {
  count = local.shouldCreate
  content         = base64decode(google_service_account_key.nomad-sa-key[count.index].private_key)
  filename        = "nomad-sa-key.json"
  file_permission = "0600"
}

resource "google_compute_disk" "csi-disk" {
  count  = var.csi_disk_count
  name   = "${var.name}-csi-disk-${count.index}"
  size   = var.csi_disk_size_gb
  type   = var.csi_disk_type
  zone   = "${var.region}-${var.zone}"
  labels = {
    environment = "dev"
  }
  physical_block_size_bytes = 4096
}

output "csi-disks" {
    description = "A map of created CSI disk resources and their self-links."
    value = zipmap(
        google_compute_disk.csi-disk[*].name,
        google_compute_disk.csi-disk[*].self_link
    )
}

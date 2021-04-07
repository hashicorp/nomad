variable "hcloud_token" {
  description = "API key"
}

variable "location" {
  default = "fsn1"
}

variable "volume_id" {
  default = "nomad-csi-test"
}

variable "hcloud_csi_driver_version" {
  description = "Driver Version"
  default     = "1.5.3"
}

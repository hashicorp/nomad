# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

variable "project" {
  type        = string
  description = "The Google Cloud Platform project to deploy the Nomad cluster in."
}

variable "credentials" {
  type        = string
  description = "The path to the Google Cloud Platform credentials file (in JSON format) to use."
}

variable "name" {
  type        = string
  default     = "hashistack"
  description = "The default name to use for resources."
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

variable "cidr_range" {
  type        = string
  default     = "192.168.1.0/24"
  description = "The IP CIDR range to use for the cluster's VPC subnetwork."
}

variable "router_asn" {
  type    = string
  default = "64514"
}

variable "image" {
  type        = string
  default     = "hashistack"
  description = "The GCP image name (built with Packer)."
}

variable "enable_preemptible" {
  type        = bool
  default     = false
  description = "Use preemptible VM instances, which will be cheaper to run."
}

variable "server_count" {
  type        = number
  default     = 3
  description = "The number of server instances to deploy (always use odd number)."
}

variable "client_count" {
  type        = number
  default     = 5
  description = "The number of client instances to deploy."
}

variable "server_machine_type" {
  type        = string
  default     = "n1-standard-2"
  description = "The compute engine machine type to use for server instances."
}

variable "client_machine_type" {
  type        = string
  default     = "n1-standard-2"
  description = "The compute engine machine type to use for client instances."
}

variable "server_disk_size_gb" {
  type        = string
  default     = "50"
  description = "The compute engine disk size in GB to use for server instances."
}

variable "client_disk_size_gb" {
  type        = string
  default     = "50"
  description = "The compute engine disk size in GB to use for client instances."
}

resource "google_compute_network" "hashistack" {
  name                    = var.name
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "hashistack" {
  network       = google_compute_network.hashistack.name
  name          = var.name
  region        = var.region
  ip_cidr_range = var.cidr_range
}

resource "google_compute_router" "hashistack" {
  name    = "${var.name}-router"
  region  = var.region
  network = google_compute_network.hashistack.name
  bgp {
    asn = var.router_asn
  }
}

resource "google_compute_router_nat" "hashistack" {
  name                               = var.name
  region                             = google_compute_router.hashistack.region
  router                             = google_compute_router.hashistack.name
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

resource "google_compute_firewall" "allow-ssh" {
  name    = "${var.name}-allow-ssh"
  network = google_compute_network.hashistack.name

  allow {
    protocol = "tcp"
    ports    = [22]
  }
}

resource "google_compute_firewall" "allow-http-external" {
  name        = "${var.name}-allow-http-external"
  network     = google_compute_network.hashistack.name
  target_tags = ["server"]

  allow {
    protocol = "tcp"
    ports    = [4646, 8200, 8500]
  }
}

resource "google_compute_firewall" "allow-all-internal" {
  name        = "${var.name}-allow-all-internal"
  network     = google_compute_network.hashistack.name
  source_tags = ["auto-join"]

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "tcp"
    ports    = ["0-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["0-65535"]
  }
}

locals {
  retry_join = "provider=gce project_name=${var.project} tag_value=auto-join"
}

locals {
  server_metadata_startup_script = <<EOF
sudo bash /ops/shared/scripts/server.sh "gce" "${var.server_count}" "${local.retry_join}"
   EOF

  client_metadata_startup_script = <<EOF
sudo bash /ops/shared/scripts/client.sh "gce" "${local.retry_join}"
   EOF
}

resource "google_compute_instance" "server" {
  count        = var.server_count
  name         = "${var.name}-server-${count.index}"
  machine_type = var.server_machine_type
  zone         = "${var.region}-${var.zone}"
  tags         = ["server", "auto-join"]

  allow_stopping_for_update = true

  boot_disk {
    initialize_params {
      image = var.image
      size  = var.server_disk_size_gb
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.hashistack.name
  }

  lifecycle {
    create_before_destroy = "true"
  }

  scheduling {
    preemptible = var.enable_preemptible
    # scheduling must have automatic_restart be false when preemptible is true.
    automatic_restart = !var.enable_preemptible
  }

  service_account {
    # https://developers.google.com/identity/protocols/googlescopes
    scopes = [
      "https://www.googleapis.com/auth/compute.readonly",
    ]
  }

  metadata = {
    enable-oslogin = true
  }

  metadata_startup_script = local.server_metadata_startup_script
}

resource "google_compute_instance" "client" {
  count        = var.client_count
  name         = "${var.name}-client-${count.index}"
  machine_type = var.client_machine_type
  zone         = "${var.region}-${var.zone}"
  tags         = ["client", "auto-join"]

  allow_stopping_for_update = true

  boot_disk {
    initialize_params {
      image = var.image
      size  = var.client_disk_size_gb
    }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.hashistack.name
  }

  lifecycle {
    create_before_destroy = "true"
  }

  scheduling {
    preemptible = var.enable_preemptible
    # scheduling must have automatic_restart be false when preemptible is true.
    automatic_restart = !var.enable_preemptible
  }

  service_account {
    # https://developers.google.com/identity/protocols/googlescopes
    scopes = [
      "https://www.googleapis.com/auth/compute.readonly",
    ]
  }

  metadata = {
    enable-oslogin = true
  }

  metadata_startup_script = local.client_metadata_startup_script
}

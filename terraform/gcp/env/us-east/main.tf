terraform {
  required_version = ">= 0.12.0"
}

variable "project" {
  type        = string
  default     = ""
  description = "The Google Cloud Platform project to deploy the Nomad cluster in."
}

variable "credentials" {
  type        = string
  default     = ""
  description = "The path to the Google Cloud Platform credentials file (in JSON format) to use."
}

variable "region" {
    type        = string
    default     = "us-east1"
    description = "The GCP region to deploy resources in."
}

variable "vm_disk_size_gb" {
  description = "The GCP disk size to use both clients and servers."
  default     = "50"
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

provider "google" {
    project     = var.project
    credentials = file(var.credentials)
}

module "hashistack" {
  source              = "../../modules/hashistack"
  project             = var.project
  credentials         = var.credentials
  server_disk_size_gb = var.vm_disk_size_gb
  server_count        = var.server_count
  client_count        = var.client_count
}

output "hashistack_load_balancer_external_ip" {
  description = "The external ip address of the HashiStack load balacner."
  value       = module.hashistack.load_balancer_external_ip
}

output "manual_config_steps" {
  value = <<CONFIGURATION
Web UI:

Nomad: http://${module.hashistack.load_balancer_external_ip}:4646/ui
Consul: http://${module.hashistack.load_balancer_external_ip}:8500/ui
Vault: http://${module.hashistack.load_balancer_external_ip}:8200/ui

Exportable CLI Environment Variables:

export NOMAD_ADDR="http://${module.hashistack.load_balancer_external_ip}:4646"
export CONSUL_HTTP_ADDR="http://${module.hashistack.load_balancer_external_ip}:8500"
export VAULT_ADDR="http://${module.hashistack.load_balancer_external_ip}:8200"

Example Commands:

  $ consul members
  $ nomad status
  $ vault status

Note:

If you see an error message like the following when running any of the above
commands, it usually indicates that the configuration script has not finished
executing:

"Error querying servers: Get http://127.0.0.1:4646/v1/agent/members: dial tcp
127.0.0.1:4646: getsockopt: connection refused"

Simply wait a few seconds and rerun the command if this occurs.
CONFIGURATION
}

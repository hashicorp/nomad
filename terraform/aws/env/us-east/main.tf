provider "aws" {
  region = var.region
}


data "hcp_packer_iteration" "hashistack-image" {
  5   bucket_name = "nomad-demo"
  6   channel     = "production"
  7 }
  8
  9 data "hcp_packer_image" "hashistack-image" {
 10   bucket_name    = "nomad-demo"
 11   cloud_provider = "aws"
 12   iteration_id   = data.hcp_packer_iteration.hashistack-image.ulid
 13   region         = "us-east-1"
 14 }


module "hashistack" {
  source = "../../modules/hashistack"

  name                   = var.name
  region                 = var.region
  ami                    = hcp_packer_image.hashistack-image.cloud_image_id
  server_instance_type   = var.server_instance_type
  client_instance_type   = var.client_instance_type
  key_name               = var.key_name
  server_count           = var.server_count
  client_count           = var.client_count
  retry_join             = var.retry_join
  nomad_binary           = var.nomad_binary
  root_block_device_size = var.root_block_device_size
  whitelist_ip           = var.whitelist_ip
}

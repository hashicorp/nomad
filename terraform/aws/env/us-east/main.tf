provider "aws" {
  region = var.region
}


data "hcp_packer_iteration" "hashistack_image" {
    bucket_name = "hashistack"
    channel     = "production"
  }
 
data "hcp_packer_image" "hashistack_image" {
  bucket_name    = "hashistack"
  cloud_provider = "aws"
  iteration_id   = data.hcp_packer_iteration.hashistack_image.ulid
  region         = "us-east-1"
}


module "hashistack" {
  source = "../../modules/hashistack"

  name                   = var.name
  region                 = var.region
  ami                    = ${hcp_packer_image.hashistack_image.cloud_image_id}
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

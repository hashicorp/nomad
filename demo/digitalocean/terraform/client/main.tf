variable "count" {}
variable "image" {}
variable "region" {}
variable "size" { default = "512mb" }
variable "servers" {}
variable "ssh_keys" {}

resource "template_file" "client_config" {
  filename = "templates/client.hcl.tpl"
  vars {
    datacenter = "${var.region}"
    servers    = "${split(",", var.servers)}"
  }
}

resource "digitalocean_droplet" "client" {
  image    = "${var.image}"
  name     = "client-${var.region}-${count.index}"
  count    = "${var.count}"
  size     = "${var.size}"
  region   = "${var.region}"
  ssh_keys = ["${split(",", var.ssh_keys)}"]

  provisioner "file" {
    source      = "${template_file.client_config.filename}"
    destination = "/usr/local/etc/nomad/client.hcl"
  }

  provisioner "remote-exec" {
    inline = ["sudo restart nomad"]
  }
}

variable "count"  {}
variable "image"  {}
variable "region" {}
variable "size"   { default = "512mb" }

resource "template_file" "server_config" {
  filename = "templates/server.hcl.tpl"
  vars {
    datacenter = "${var.region}"
  }
}

resource "digitalocean_droplet" "server" {
  image  = "${var.image}"
  name   = "server-${var.region}-${count.index}"
  count  = "${var.count}"
  size   = "${var.size}"
  region = "${var.region}"

  provisioner "file" {
    source      = "${template_file.server_config.filename}"
    destination = "/usr/local/etc/nomad/server.hcl"
  }

  provisioner "remote-exec" {
    inline = ["sudo restart nomad"]
  }
}

output "addrs" {
  value = "${join(",", digitalocean_droplet.server.*.ipv4_address)}"
}

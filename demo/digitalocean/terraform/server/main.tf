variable "count"  {}
variable "image"  {}
variable "region" {}
variable "size"   {}

resource "template_file" "server_config" {
  filepath = "templates/server.hcl.tpl"
  vars {
    datacenter = "${var.region}"
  }
}

resource "digitalocean_droplet" "server" {
  image = "${var.image}"
  name  = "server-${var.region}-${count.index}"
  count = "${var.count}"
  size  = "${var.size}"

  provisioner "file" {
    source      = "${template_file.server_config.filename}"
    destination = "/usr/local/etc/nomad/server.hcl"
  }

  provisioner "remote-exec" {
    inline = ["sudo restart nomad"]
  }
}

output "addrs" {
  value = "${join(",", resource.digitalocean_droplet.*.ipv4_address)}"
}

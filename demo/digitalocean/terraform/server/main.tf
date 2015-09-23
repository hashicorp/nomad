variable "count" {}
variable "image" {}
variable "region" {}
variable "size" { default = "512mb" }
variable "ssh_keys" {}

resource "template_file" "server_config" {
  filename = "templates/server.hcl.tpl"
  vars {
    datacenter = "${var.region}"
  }
}

resource "digitalocean_droplet" "server" {
  image    = "${var.image}"
  name     = "server-${var.region}-${count.index}"
  count    = "${var.count}"
  size     = "${var.size}"
  region   = "${var.region}"
  ssh_keys = ["${split(",", var.ssh_keys)}"]

  provisioner "remote-exec" {
    inline = ["cat > /usr/local/etc/nomad/server.hcl <<EOF
${template_file.server_config.rendered}
EOF"]
  }

  provisioner "remote-exec" {
    inline = ["sudo restart nomad"]
  }
}

resource "null_resource" "server_join" {
  provisioner "local-exec" {
    command = <<EOF
join() {
  curl -X PUT ${digitalocean_droplet.server.0.ipv4_address}:4646/v1/agent/join?address=$1
}
join ${digitalocean_droplet.server.1.ipv4_address}
join ${digitalocean_droplet.server.2.ipv4_address}
EOF
  }
}

output "addrs" {
  value = "${join(",", digitalocean_droplet.server.*.ipv4_address)}"
}

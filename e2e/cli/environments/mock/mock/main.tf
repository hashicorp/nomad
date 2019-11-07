variable "nomad_binary" {}

output "nomad_addr" {
  value = "${var.nomad_binary}"
}

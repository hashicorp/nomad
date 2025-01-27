variable "nomad_addr" {
  description = "The Nomad API HTTP address."
  type        = string
  default     = "http://localhost:4646"
}

variable "ca_file" {
  description = "A local file path to a PEM-encoded certificate authority used to verify the remote agent's certificate"
  type        = string
}

variable "cert_file" {
  description = "A local file path to a PEM-encoded certificate provided to the remote agent. If this is specified, key_file or key_pem is also required"
  type        = string
}

variable "key_file" {
  description = "A local file path to a PEM-encoded private key. This is required if cert_file or cert_pem is specified."
  type        = string
}

variable "nomad_token" {
  description = "The Secret ID of an ACL token to make requests with, for ACL-enabled clusters."
  type        = string
}

variable "platform" {
  description = "Operative system of the instance to upgrade"
  type        = string
  default     = "linux"
}

variable "server_address" {
  description = "IP address of the server tha will be updated"
  type        = string
  default     = "107.21.82.251"
}

variable "nomad_local_upgrade_binary" {
  description = "The path to a local binary to upgrade"
}

variable "ssh_key_path" {
  description = "Path to the ssh private key that can be used to connect to the instance where the server is running"
  type        = string
}

variable "nomad_remote_binary" {
  description = "Path were the nomad binary will be placed to replace the old one on the server"
}



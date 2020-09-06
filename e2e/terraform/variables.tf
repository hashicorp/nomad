variable "name" {
  description = "Used to name various infrastructure components"
  default     = "nomad-e2e"
}

variable "region" {
  description = "The AWS region to deploy to."
  default     = "us-east-1"
}

variable "availability_zone" {
  description = "The AWS availability zone to deploy to."
  default     = "us-east-1a"
}

variable "indexed" {
  description = "Different configurations per client/server"
  default     = true
}

variable "instance_type" {
  description = "The AWS instance type to use for both clients and servers."
  default     = "t2.medium"
}

variable "server_count" {
  description = "The number of servers to provision."
  default     = "3"
}

variable "client_count" {
  description = "The number of clients to provision."
  default     = "4"
}

variable "windows_client_count" {
  description = "The number of windows clients to provision."
  default     = "1"
}

variable "nomad_sha" {
  description = "The sha of Nomad to write to provisioning output"
  default     = ""
}

variable "aws_assume_role_arn" {
  description = "The AWS IAM role to assume (not used by human users)"
  default     = ""
}

variable "aws_assume_role_session_name" {
  description = "The AWS IAM session name to assume (not used by human users)"
  default     = ""
}

variable "aws_assume_role_external_id" {
  description = "The AWS IAM external ID to assume (not used by human users)"
  default     = ""
}

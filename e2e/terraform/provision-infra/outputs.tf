# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

output "servers" {
  value = aws_instance.server.*.public_ip
}

output "linux_clients" {
  value = aws_instance.client_ubuntu_jammy.*.public_ip
}

output "windows_clients" {
  value = aws_instance.client_windows_2016.*.public_ip
}

output "clients" {
  value = concat(aws_instance.client_ubuntu_jammy.*.public_ip, aws_instance.client_windows_2016.*.public_ip)
}

output "message" {
  value = <<EOM
Your cluster has been provisioned! To prepare your environment, run:

   $(terraform output --raw environment)

Then you can run tests from the e2e directory with:

   go test -v .

ssh into servers with:

%{for ip in aws_instance.server.*.public_ip~}
   ssh -i ${local.keys_dir}/${local.random_name}.pem ubuntu@${ip}
%{endfor~}

ssh into clients with:

%{for ip in aws_instance.client_ubuntu_jammy.*.public_ip~}
    ssh -i ${local.keys_dir}/${local.random_name}.pem ubuntu@${ip}
%{endfor~}
%{for ip in aws_instance.client_windows_2016.*.public_ip~}
    ssh -i ${local.keys_dir}/${local.random_name}.pem Administrator@${ip}
%{endfor~}

EOM
}

# Note: Consul and Vault environment needs to be set in test
# environment before the Terraform run, so we don't have that output
# here
output "environment" {
  description = "get connection config by running: $(terraform output environment)"
  sensitive   = true
  value       = <<EOM
export NOMAD_ADDR=https://${aws_instance.server[0].public_ip}:4646
export NOMAD_CACERT=${abspath(local.keys_dir)}/tls_ca.crt
export NOMAD_CLIENT_CERT=${abspath(local.keys_dir)}/tls_api_client.crt
export NOMAD_CLIENT_KEY=${abspath(local.keys_dir)}/tls_api_client.key
export NOMAD_TOKEN=${data.local_sensitive_file.nomad_token.content}
export NOMAD_E2E=1
export CONSUL_HTTP_ADDR=https://${aws_instance.consul_server.public_ip}:8501
export CONSUL_HTTP_TOKEN=${local_sensitive_file.consul_initial_management_token.content}
export CONSUL_CACERT=${abspath(local.keys_dir)}/tls_ca.crt
export CLUSTER_UNIQUE_IDENTIFIER=${local.random_name}
EOM
}

output "cluster_unique_identifier" {
  value = "${local.random_name}"
}

output "nomad_addr" {
  value = "https://${aws_instance.server[0].public_ip}:4646"
}

output "ca_file" {
  value = "${abspath(local.keys_dir)}/tls_ca.crt"
}

output "cert_file" {
  value = "${abspath(local.keys_dir)}/tls_api_client.crt"
}

output "key_file" {
  value = "${abspath(local.keys_dir)}/tls_api_client.key"
}

output "ssh_key_file" {
  value = "${abspath(local.keys_dir)}/${local.random_name}.pem"
}

output "nomad_token" {
  value     = chomp(data.local_sensitive_file.nomad_token.content)
  sensitive = true
}

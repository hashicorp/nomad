#output "ipv6" {
#  value = local.server_addrs
#}

output "env" {
  value = <<EOF
export NOMAD_ADDR=http://[${local.server_addrs[0]}]:4646
EOF
}

output "ssh" {
  value = <<EOF
%{for ip in flatten(aws_instance.mine.*.ipv6_addresses)~}
ssh -i ${module.keys.private_key_filepath} ec2-user@${ip}
%{endfor~}
EOF
}

output "ssh_config" {
  value = <<EOF
Host aws-*
    User ec2-user
    IdentityFile ${abspath(module.keys.private_key_filepath)}
    StrictHostKeyChecking no
    UserKnownHostsFile=/dev/null
%{for i, ip in flatten(aws_instance.mine.*.ipv6_addresses)~}
Host aws-${i}
    Hostname ${ip}
%{endfor~}
EOF
}


#output "ipv4" {
#  value = aws_instance.mine.*.public_ip
#}
#
#output "ipv6" {
#  value = aws_instance.mine.*.ipv6_addresses
#}

output "ssh" {
  value = <<EOF
%{for i1, ips in aws_instance.mine.*.ipv6_addresses~}
%{~for i2, ip in ips ~}
ssh -i ${module.keys.private_key_filepath} ec2-user@${ip}
%{endfor~}
%{endfor~}
EOF
}

output "ssh_config" {
  value = <<EOF
%{for i1, ips in aws_instance.mine.*.ipv6_addresses~}
%{~for i2, ip in ips ~}
Host aws-${i1 + i2}
    Hostname ${ip}
    User ec2-user
    IdentityFile ${abspath(module.keys.private_key_filepath)}
%{endfor~}
%{endfor~}
EOF
}

output "env" {
  value = <<EOF
export NOMAD_ADDR=http://[${local.server_addrs[0]}]:4646
EOF
}

#output "ssh" {
#  value = <<-EOF
#    %{for ip in aws_instance.mine.*.public_ip~}
#    ssh -i ${module.keys.private_key_filepath} ec2-user@${ip}
#    %{endfor~}
#    EOF
#}
#
#output "ssh_config" {
#  value = <<-EOF
#    %{for idx, ip in aws_instance.mine.*.public_ip~}
#    Host aws${idx}
#    	Hostname ${ip}
#    	User ec2-user
#    	IdentityFile ${module.keys.private_key_filepath}
#    %{endfor~}
#    EOF
#}

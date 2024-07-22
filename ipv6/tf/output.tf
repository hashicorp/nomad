output "ipv4" {
  value = aws_instance.mine.public_ip
}

output "ipv6" {
  value = aws_instance.mine.ipv6_addresses
}

output "ssh" {
  value = "ssh -i ${var.pubkey} ec2-user@${aws_instance.mine.public_ip}"
}


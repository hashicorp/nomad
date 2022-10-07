output "IP_Addresses" {
  value = <<CONFIGURATION

Client public IPs: ${join(", ", module.hashistack.client_public_ips)}\n
\n
Server public IPs: ${join(", ", module.hashistack.server_public_ips)}\n
\n
To connect, add your private key and SSH into any client or server with\n
`ssh ubuntu@PUBLIC_IP`. You can test the integrity of the cluster by running:\n
\n
  $ consul members\n
  $ nomad server members\n
  $ nomad node status\n
\n
If you see an error message like the following when running any of the above\n
commands, it usually indicates that the configuration script has not finished\n
executing:\n
\n
"Error querying servers: Get http://127.0.0.1:4646/v1/agent/members: dial tcp\n
127.0.0.1:4646: getsockopt: connection refused"\n
\n
Simply wait a few seconds and rerun the command if this occurs.\n
\n
The Nomad UI can be accessed at http://${module.hashistack.server_lb_ip}:4646/ui.\n
The Consul UI can be accessed at http://${module.hashistack.server_lb_ip}:8500/ui.\n
\n
Set the following for access from the Nomad CLI:\n
\n
  export NOMAD_ADDR=http://${module.hashistack.server_lb_ip}:4646\n
\n
CONFIGURATION

}

region                           = "us-east-1"
instance_type                    = "t3.medium"
server_count                     = "1"
client_count_ubuntu_bionic_amd64 = "1"
client_count_windows_2016_amd64  = "1"
profile                          = "dev-cluster"
nomad_acls                       = false
nomad_enterprise                 = false
vault                            = true
volumes                          = true
tls = true

nomad_version      = "1.0.1"  # default version for deployment
nomad_sha          = ""       # overrides nomad_version if set
nomad_local_binary = ""       # overrides nomad_sha and nomad_version if set

# Example overrides:
# nomad_sha = "38e23b62a7700c96f4898be777543869499fea0a"
# nomad_local_binary = "../../pkg/linux_amd/nomad"
# nomad_local_binary_client_windows_2016_amd64 = ["../../pkg/windows_amd64/nomad.exe"]

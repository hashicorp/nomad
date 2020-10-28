region                           = "us-east-1"
instance_type                    = "t3.medium"
server_count                     = "3"
client_count_ubuntu_bionic_amd64 = "2"
client_count_windows_2016_amd64  = "0"
profile                          = "dev-cluster"
nomad_acls                       = false
nomad_enterprise                 = false
vault                            = true
volumes                          = false

# Example overrides:
# nomad_local_binary = "../../pkg/linux_amd/nomad"
# nomad_local_binary_client_windows_2016_amd64 = ["../../pkg/windows_amd64/nomad.exe"]

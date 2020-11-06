region                           = "us-east-1"
instance_type                    = "t3.medium"
server_count                     = "3"
client_count_ubuntu_bionic_amd64 = "5"
client_count_windows_2016_amd64  = "0"
profile                          = "custom"
nomad_acls                       = false
nomad_enterprise                 = false
vault                            = false
volumes                          = false

# nomad_version = "1.0.0-beta2" # default version for deployment
# nomad_sha = "5718115938f7cae5eb119a54a1e1c4f02748f8a0" # overrides nomad_version if set
nomad_local_binary = "../../pkg/linux_amd64/nomad" # overrides nomad_sha and nomad_version if set
# Example overrides:
# nomad_sha = "38e23b62a7700c96f4898be777543869499fea0a"
# nomad_local_binary = "../../pkg/linux_amd64/nomad"
# nomad_local_binary_client_windows_2016_amd64 = ["../../pkg/windows_amd64/nomad.exe"]

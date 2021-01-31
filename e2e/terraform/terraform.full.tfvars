region                           = "us-east-1"
instance_type                    = "t3.medium"
server_count                     = "3"
client_count_ubuntu_bionic_amd64 = "4"
client_count_windows_2016_amd64  = "1"
profile                          = "full-cluster"
nomad_enterprise                 = true
nomad_acls                       = true
vault                            = true
volumes                          = true

# required to avoid picking up defaults from terraform.tfvars file
nomad_version      = "" # default version for deployment
nomad_sha          = "" # overrides nomad_version if set
nomad_local_binary = "" # overrides nomad_sha and nomad_version if set

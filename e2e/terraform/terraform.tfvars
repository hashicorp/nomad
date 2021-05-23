region                           = "us-east-1"
instance_type                    = "t3.medium"
server_count                     = "3"
client_count_ubuntu_bionic_amd64 = "2"
client_count_windows_2016_amd64  = "0"
profile                          = "dev-cluster"
nomad_acls                       = false
nomad_enterprise                 = false
vault                            = false
volumes                          = false

nomad_version      = "" # default version for deployment
nomad_local_binary = ""      # overrides nomad_version if set
nomad_url = "https://156691-36653430-gh.circle-artifacts.com/0/builds/linux_amd64.zip"

# Example overrides:
# nomad_local_binary = "../../pkg/linux_amd64/nomad"
# nomad_local_binary_client_windows_2016_amd64 = ["../../pkg/windows_amd64/nomad.exe"]

# The nightly E2E runner will set a nomad_sha flag; this should not be used
# outside of the nightly E2E runner and will usually fail because the build
# will not be available

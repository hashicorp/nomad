# test: install a specific Nomad version with indexed configs
profile = "full-cluster"

server_count         = 3
client_count         = 4
windows_client_count = 1

nomad_version = "0.12.1"
nomad_version_server = [
  "0.12.0", # override servers 1 and 2
  "0.12.3",
]
nomad_version_client_linux = [
  "0.12.0", # override clients 1 and 2
  "0.12.3",
]
nomad_version_client_windows = [
  "0.12.0", # override windows client
  "0.12.3", # ignored
]

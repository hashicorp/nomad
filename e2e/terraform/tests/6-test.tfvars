# test: install a local Nomad binary, with overrides
profile = "dev-cluster"

server_count         = 3
client_count         = 4
windows_client_count = 1

nomad_local_binary = "./mock-1"
nomad_local_binary_server = [
  "./mock-2", # override servers 1 and 2
  "./mock-2",
]
nomad_local_binary_client_linux = [
  "./mock-2" # override client 1
]
nomad_local_binary_client_windows = [
  "./mock-2" # override windows client
]

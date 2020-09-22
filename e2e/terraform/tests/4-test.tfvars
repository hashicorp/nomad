# test: install a specific Nomad sha with overrides
profile = "dev-cluster"

server_count         = 3
client_count         = 4
windows_client_count = 1

nomad_sha = "2a6e62be00a0db228d8add74ceca6ca83c8efdcf"
nomad_sha_server = [
  "920f00da22726914e504d016bb588ca9c18240f2", # override server 1 and 2
  "568c4aa72b51050913365dae6b3b1d089d39b2a5",
]
nomad_sha_client_linux = [
  "920f00da22726914e504d016bb588ca9c18240f2", # override client 1 and 2
  "568c4aa72b51050913365dae6b3b1d089d39b2a5",
]
nomad_sha_client_windows = [
  "920f00da22726914e504d016bb588ca9c18240f2", # override windows client
  "568c4aa72b51050913365dae6b3b1d089d39b2a5", # ignored
]

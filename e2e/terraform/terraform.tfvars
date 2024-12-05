# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this default tfvars file expects that you have built nomad
# with `make dev` or similar (../../ = this repository root)
# before running `terraform apply` and created the /pkg/goos_goarch/binary 
# folder

nomad_local_binary                           = "/Users/juanita.delacuestamorales/nomad/linux-amd64-ce/nomad"
nomad_local_binary_client_windows_2016_amd64 = ["../../pkg/windows_amd64/nomad.exe"]
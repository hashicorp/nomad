# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

# this default tfvars file expects that you have built nomad
# with `make dev` or similar (../../ = this repository root)
# before running `terraform apply`

nomad_local_binary                           = "../../pkg/linux_amd64/nomad"
nomad_local_binary_client_windows_2016_amd64 = ["../../pkg/windows_amd64/nomad.exe"]

# For testing enterprise, set via --var:
# nomad_license = <content of Nomad license>

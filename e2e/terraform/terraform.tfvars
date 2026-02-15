# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

# this default tfvars file expects that you have built nomad
# with `make dev` or similar (../../ = this repository root)
# before running `terraform apply` and created the /pkg/goos_goarch/binary
# folder
#
# For the device e2e tests, also build the example device plugin:
#   make pkg/linux_amd64/nomad-device-example

nomad_local_binary                     = "../../pkg/linux_amd64/nomad"
nomad_local_binary_client_windows_2022 = "../../pkg/windows_amd64/nomad.exe"
device_plugin_local_binary             = "../../pkg/linux_amd64/nomad-device-example"

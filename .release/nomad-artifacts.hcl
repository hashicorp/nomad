# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

schema = 1
artifacts {
  zip = [
    "nomad_${version}_darwin_amd64.zip",
    "nomad_${version}_darwin_arm64.zip",
    "nomad_${version}_linux_amd64.zip",
    "nomad_${version}_linux_arm64.zip",
    "nomad_${version}_windows_amd64.zip",
  ]
  rpm = [
    "nomad-${version_linux}-1.aarch64.rpm",
    "nomad-${version_linux}-1.x86_64.rpm",
  ]
  deb = [
    "nomad_${version_linux}-1_amd64.deb",
    "nomad_${version_linux}-1_arm64.deb",
  ]
  container = [
    "nomad_release_linux_amd64_${version}_${commit_sha}.docker.dev.tar",
    "nomad_release_linux_amd64_${version}_${commit_sha}.docker.tar",
    "nomad_release_linux_arm64_${version}_${commit_sha}.docker.dev.tar",
    "nomad_release_linux_arm64_${version}_${commit_sha}.docker.tar",
  ]
}

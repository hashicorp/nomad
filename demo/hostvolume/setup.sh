#!/usr/bin/env bash
# Copyright IBM Corp. 2015, 2025
# SPDX-License-Identifier: BUSL-1.1

set -xeuo pipefail

nomad volume create external-plugin.volume.hcl
nomad volume create internal-plugin.volume.hcl

nomad job run job.nomad.hcl

nomad volume status -type=host -verbose
nomad operator api /v1/nodes | jq '.[].HostVolumes'


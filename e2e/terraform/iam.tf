# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# note: the creation of this instance profile is in a HashiCorp private repo
data "aws_iam_instance_profile" "nomad_e2e_cluster" {
  name = "nomad_e2e_cluster"
}

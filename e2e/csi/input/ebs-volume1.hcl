# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: BUSL-1.1

id        = "ebs-vol[1]"
name      = "idempotency-token" # CSIVolumeName tag
type      = "csi"
plugin_id = "aws-ebs0"

capacity_min = "10GiB"
capacity_max = "20GiB"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "block-device"
}

parameters {
  type = "gp2"
}

topology_request {
  required {
    topology {
      segments {
        # this zone should match the one set in e2e/terraform/variables.tf
        "topology.ebs.csi.aws.com/zone" = "us-east-1b"
      }
    }
  }
}

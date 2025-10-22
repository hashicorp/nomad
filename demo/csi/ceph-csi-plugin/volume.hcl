# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

id        = "testvolume"
name      = "2e1064ef-4ed3-48a8-af27-9d29611ee967"
type      = "csi"
plugin_id = "cephrbd"

capacity_min = "100MB"
capacity_max = "1GB"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

# NOTE: these are bogus test secrets pulled from the demo ceph container's
# keyring file, not real secrets that we care about securing
secrets {
  userID  = "admin"
  userKey = "AQBN3PdoU2vbCBAA3z4eAts5FwEpwmM0B+AEaA=="
}

# NOTE: we have to update this on each run, would be good to be able to template
# these via HCL2
parameters {
  clusterID     = "540cdda2-84fe-4a41-8115-59c79b450536"
  pool          = "rbd"
  imageFeatures = "layering"
  fsName        = "foo"
}

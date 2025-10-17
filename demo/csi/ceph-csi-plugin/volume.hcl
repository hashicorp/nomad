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

# capability {
#   access_mode     = "single-node-writer"
#   attachment_mode = "block-device"
# }

# mount_options {
#   fs_type     = "ext4"
#   mount_flags = ["ro"]
# }


# creds should be coming from:
# /var/lib/ceph/mds/ceph-demo/keyring

# but instead we're getting them from:
# /etc/ceph/ceph.client.admin.keyring

secrets {
  userID  = "YWRtaW4="
  userKey = "QVFEalFzMW9RTGhDSWhBQTBBZHpHdFFzL01XVUhaTUZtRDFaaXc9PQ=="
}
# secrets {
#   userID  = "admin"
#   userKey = "AQBN9Mpoycc9JhAAQmzirsWH7k2U7x74hFVcWA=="
# }

parameters {
  clusterID     = "0e019840-717a-4e45-a192-d9adf3d7410c"
  pool          = "rbd"
  imageFeatures = "layering"
  fsName        = "foo"
}

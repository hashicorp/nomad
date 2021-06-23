id        = "testvolume"
name      = "test1"
type      = "csi"
plugin_id = "cephrbd"

capacity_min = "100MB"
capacity_max = "1GB"

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "file-system"
}

capability {
  access_mode     = "single-node-writer"
  attachment_mode = "block-device"
}

# mount_options {
#   fs_type     = "ext4"
#   mount_flags = ["ro"]
# }


# creds should be coming from:
# /var/lib/ceph/mds/ceph-demo/keyring

# but instead we're getting them from:
# /etc/ceph/ceph.client.admin.keyring

secrets {
  userID  = "admin"
  userKey = "AQDsIoxgHqpeBBAAtmd9Ndu4m1xspTbvwZdIzA=="
}

parameters {
  # seeded from uuid5(ceph.example.com)
  clusterID     = "e9ba69fa-67ff-5920-b374-84d5801edd19"
  pool          = "rbd"
  imageFeatures = "layering"
}

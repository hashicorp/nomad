type = "csi"
id   = "testvol"
name = "test_volume"
# this must be strictly formatted, see README
external_id     = "ffff-0024-01616094-9d93-4178-bf45-c7eac19e8b15-000000000000ffff-00000000-1111-2222-bbbb-cacacacacaca"
access_mode     = "single-node-writer"
attachment_mode = "block-device"
plugin_id       = "ceph-csi"
mount_options {
  fs_type = "ext4"
}
parameters {}
secrets {
  userID  = "<userid>"
  userKey = "<userkey>"
}
context {
  # note: although these are 'parameters' in the ceph-csi spec
  # they are passed through to the provider as 'context'
  clusterID = "<clusterid>"
  pool      = "my_pool"
}
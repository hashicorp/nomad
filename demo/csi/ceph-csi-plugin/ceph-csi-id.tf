locals {
  # ClusterID: Is a unique ID per cluster that the CSI instance is serving and is restricted to
  # lengths that can be accommodated in the encoding scheme.
  # must be less than 128 chars. must match the cluster id in the csi plugin conf.
  ClusterID = "<clusterid>"

  # EncodingVersion: Carries the version number of the encoding scheme used to encode the CSI ID,
  # and is preserved for any future proofing w.r.t changes in the encoding scheme, and to retain
  # ability to parse backward compatible encodings.
  # https://github.com/ceph/ceph-csi/blob/ef1785ce4db0aa1f6878c770893bcabc71cff300/internal/cephfs/driver.go#L31
  EncodingVersion = 1

  # LocationID: 64 bit integer identifier determining the location of the volume on the Ceph cluster.
  # It is the ID of the poolname or fsname, for RBD or CephFS backed volumes respectively.
  # see https://docs.ceph.com/docs/mimic/rbd/rados-rbd-cmds/
  LocationID = 7

  # ObjectUUID: Is the on-disk uuid of the object (image/snapshot) name, for the CSI volume that
  # corresponds to this CSI ID.. must be 36 chars long.
  ObjectUUID = "abcd"
}

data "template_file" "csi_id" {
  template = "$${versionEncodedHex}-$${clusterIDLength}-$${ciClusterID}-$${poolIDEncodedHex}-$${ciObjectUUID}"

  vars = {
    versionEncodedHex = "${format("%02X", local.EncodingVersion)}"
    clusterIDLength   = "${format("%02X", length(local.ClusterID))}"
    ciClusterID       = "${local.ClusterID}"
    poolIDEncodedHex  = "${format("%016X", local.LocationID)}"
    ciObjectUUID      = "${local.ObjectUUID}"
  }
}

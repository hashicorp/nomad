# Openstack Ceph-CSI Plugin

The configuration here is for the Ceph RBD driver, migrated from the k8s config [documentation](https://github.com/ceph/ceph-csi/blob/master/docs/deploy-rbd.md). It can be easily modified for the CephFS Driver, as used [here](https://github.com/ceph/ceph-csi/blob/master/docs/deploy-cephfs.md).

## Requirements

The example plugin job creates a file at `local/cloud.conf` using a [`template`](https://www.nomadproject.io/docs/job-specification/template) stanza which pulls the necessary credentials from a [Vault kv-v2](https://www.vaultproject.io/docs/secrets/kv/kv-v2) secrets store. 


### Docker Privileged Mode

The Ceph CSI Node task requires that [`privileged = true`](https://www.nomadproject.io/docs/drivers/docker#privileged) be set. This is not needed for the Controller task.

## Container Arguments
 
Refer to the official plugin [guide](https://github.com/ceph/ceph-csi/blob/master/docs/deploy-rbd.md).
 
- `--type=rbd`
 
  - Driver type `rbd` (or alternately `cephfs`)

- `--endpoint=unix:///csi/csi.sock`

  - This option must match the `mount_dir` specified in the `csi_plugin` stanza for the task.

- `--nodeid=${node.unique.name}`

  - A unique ID for the node the task is running on. Recommend using `${node.unique.name}`

- `--cluster=${NOMAD_DC}`

  - The cluster the Controller/Node is a part of. Recommend using `${NOMAD_DC}`

- `--instanceid=${attr.unique.platform.aws.instance-id}`
  
  - Unique ID distinguishing this instance of Ceph CSI among other instances, when sharing Ceph clusters across CSI instances for provisioning. Used for topology-aware deployments.

## Deployment

### Plugin

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad job run ceph-csi-plugin.hcl
```

### Volume Registration

The `external_id` value for the volume must be strictly formatted, see `ceph_csi.tf`. Based on [Ceph-CSI ID Format](https://github.com/ceph/ceph-csi/blob/71ddf51544be498eee03734573b765eb04480bb9/internal/util/volid.go#L27), see [examples](https://github.com/ceph/ceph-csi/blob/71ddf51544be498eee03734573b765eb04480bb9/internal/util/volid_test.go#L33).

The `secrets` block will be populated with values pulled from `/etc/ceph/ceph.client.<user>.keyring`, e.g.
```
userid = "<user>"
userkey = "AWBg/BtfJInSFBATOrrnCh6UGE3QB3nYakdF+g=="
```

```bash
export NOMAD_ADDR=https://nomad.example.com:4646
export NOMAD_TOKEN=34534-3sdf3-szfdsafsdf3423-zxdfsd3
nomad volume register example_volume.hcl
```

## Ceph CSI Driver Source

- https://github.com/ceph/ceph-csi

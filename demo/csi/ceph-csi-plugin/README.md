# Ceph CSI Plugin

The configuration here is for the Ceph RBD driver, migrated from the k8s
config
[documentation](https://github.com/ceph/ceph-csi/blob/master/docs/deploy-rbd.md). It
can be modified for the CephFS Driver, as used
[here](https://github.com/ceph/ceph-csi/blob/master/docs/deploy-cephfs.md).

## Deployment

The Ceph CSI Node task requires that [`privileged =
true`](https://developer.hashicorp.com/nomad/docs/drivers/docker#privileged) be
set. This is not needed for the Controller task.

### Plugin Arguments

Refer to the official plugin
[guide](https://github.com/ceph/ceph-csi/blob/master/docs/deploy-rbd.md).

* `--type=rbd`: driver type `rbd` (or alternately `cephfs`)

* `--endpoint=${CSI_ENDPOINT}`: if you don't use the `CSI_ENDPOINT`
    environment variable, this option must match the `mount_dir`
    specified in the `csi_plugin` block for the task.

* `--nodeid=${node.unique.id}`: a unique ID for the node the task is running
  on.

* `--instanceid=${NOMAD_ALLOC_ID}`: a unique ID distinguishing this instance
    of Ceph CSI among other instances, when sharing Ceph clusters across CSI
    instances for provisioning. Used for topology-aware deployments.

### Run the Plugins

Run the plugins:

```
$ nomad job run -var-file=nomad.vars ./plugin-cephrbd-controller.nomad
==> Monitoring evaluation "c8e65575"
    Evaluation triggered by job "plugin-cephrbd-controller"
==> Monitoring evaluation "c8e65575"
    Evaluation within deployment: "b15b6b2b"
    Allocation "1955d2ab" created: node "8dda4d46", group "cephrbd"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "c8e65575" finished with status "complete"

$ nomad job run -var-file=nomad.vars ./plugin-cephrbd-node.nomad
==> Monitoring evaluation "5e92c5dc"
    Evaluation triggered by job "plugin-cephrbd-node"
==> Monitoring evaluation "5e92c5dc"
    Allocation "5bb9e57a" created: node "8dda4d46", group "cephrbd"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "5e92c5dc" finished with status "complete"

$ nomad plugin status cephrbd
ID                   = cephrbd
Provider             = rbd.csi.ceph.com
Version              = canary
Controllers Healthy  = 1
Controllers Expected = 1
Nodes Healthy        = 1
Nodes Expected       = 1

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created    Modified
1955d2ab  8dda4d46  cephrbd     0        run      running  3m47s ago  3m37s ago
5bb9e57a  8dda4d46  cephrbd     0        run      running  3m44s ago  3m43s ago
```

### Create a Volume

The `secrets` block for the volume must be populated with the `userID` and
`userKey` values pulled from `/etc/ceph/ceph.client.<user>.keyring`.

```
$ nomad volume create ./volume.hcl
Created external volume 0001-0024-e9ba69fa-67ff-5920-b374-84d5801edd19-0000000000000002-3603408d-a9ca-11eb-8ace-080027c5bc64 with ID testvolume
```

### Register a Volume

You can register a volume that already exists in Ceph. In this case, you'll
need to provide the `external_id` field. The `ceph-csi-id.tf` Terraform file
in this directory can be used to generate the correctly-formatted ID. This is
based on [Ceph-CSI ID
Format](https://github.com/ceph/ceph-csi/blob/71ddf51544be498eee03734573b765eb04480bb9/internal/util/volid.go#L27)
(see
[examples](https://github.com/ceph/ceph-csi/blob/71ddf51544be498eee03734573b765eb04480bb9/internal/util/volid_test.go#L33)).


## Running Ceph in Vagrant

For demonstration purposes only, you can run Ceph as a single container Nomad
job on the Vagrant VM managed by the `Vagrantfile` at the top-level of this
repo.

The `./run-ceph.sh` script in this directory will deploy the demo container
and wait for it to be ready. The data served by this container is entirely
ephemeral and will be destroyed once it stops; you should not use this an
example of how to run production Ceph workloads!

```sh
$ ./run-ceph.sh

nomad job run -var-file=nomad.vars ./ceph.nomad
==> Monitoring evaluation "68dde586"
    Evaluation triggered by job "ceph"
==> Monitoring evaluation "68dde586"
    Evaluation within deployment: "79e23968"
    Allocation "77fd50fb" created: node "ca3ee034", group "ceph"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "68dde586" finished with status "complete"

waiting for Ceph to be ready..............................
ready!
```

The setup script in the Ceph container configures a key, which you'll need for
creating volumes. You can extract the key from the keyring via `nomad alloc
exec`:

```
$ nomad alloc exec 77f  cat /etc/ceph/ceph.client.admin.keyring | awk '/key/{print $3}'
AQDsIoxgHqpeBBAAtmd9Ndu4m1xspTbvwZdIzA==
```

To run the Controller plugin against this Ceph, you'll need to use the plugin
job in the file `plugin-cephrbd-controller-vagrant.nomad` so that it can reach
the correct ports.

## Ceph CSI Driver Source

- https://github.com/ceph/ceph-csi

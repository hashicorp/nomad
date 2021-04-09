# Hostpath CSI Plugin

This directory includes a demo using the [CSI host path
driver](https://github.com/kubernetes-csi/csi-driver-host-path) to create
local "host path" volumes that can be mounted via the Nomad CSI
implementation.

## What Is This For?

The hostpath plugin is for demonstration and development purposes only. It
shouldn't be used for production. If you want to get a quick idea of how CSI
works on Nomad in a Vagrant environment, this demo is a good option.

## Requirements

* A running Nomad cluster with `docker.privileged.enabled = true` and
  `docker.volumes.enabled = true`. The Nomad developer
  [Vagrantfile](https://github.com/hashicorp/nomad/blob/main/Vagrantfile) in
  this repo is suitable.

Running the `run.sh` script in this directory will output the Nomad command
used to run the demo, as well as their outputs:

```
$ nomad job run ./plugin.nomad
==> Monitoring evaluation "7ac3cc8d"
    Evaluation triggered by job "csi-plugin"
    Allocation "bbd34b72" created: node "917b009b", group "csi"
==> Monitoring evaluation "7ac3cc8d"
    Allocation "bbd34b72" status changed: "pending" -> "running" (Tasks are running)
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "7ac3cc8d" finished with status "complete"
Nodes Healthy        = 1

$ nomad plugin status hostpath
ID                   = hostpath-plugin0
Provider             = csi-hostpath
Version              = v1.2.0-0-g83590990
Controllers Healthy  = 1
Controllers Expected = 1
Nodes Healthy        = 1
Nodes Expected       = 1

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created  Modified
bbd34b72  917b009b  csi         0        run      running  3s ago   2s ago

$ cat hostpath.hcl | sed | nomad volume create -
Created external volume 7185cd16-993f-11eb-a052-0242ac110002 with ID test-volume[0]

$ cat hostpath.hcl | sed | nomad volume create -
Created external volume 718bd6b4-993f-11eb-a052-0242ac110002 with ID test-volume[1]

$ nomad job run ./redis.nomad
==> Monitoring evaluation "3178513e"
    Evaluation triggered by job "example"
    Evaluation within deployment: "ffb161f4"
    Allocation "139caa78" created: node "917b009b", group "cache"
    Allocation "5e1b57f5" created: node "917b009b", group "cache"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "3178513e" finished with status "complete"

$ nomad volume status
Container Storage Interface
ID              Name            Plugin ID         Schedulable  Access Mode
test-volume[0]  test-volume[0]  hostpath-plugin0  true         single-node-reader-only
test-volume[1]  test-volume[1]  hostpath-plugin0  true         single-node-reader-only
```

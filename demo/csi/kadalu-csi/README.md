# Kadalu CSI Plugin

- The configuration here is for using external gluster volumes as persistent storage in Nomad using Kadalu CSI
- Please refer actual job files before proceeding with demo and change config as required and follow along with commands according to your config
- Locally tested against Nomad v1.1.4

## Local Development

- This section can be skipped if you already have a Nomad cluster setup

``` sh
# Clone config repo used to create local Nomad cluster in docker and install shipyard following it's README
-> git clone https://github.com/leelavg/kadalu-nomad && cd kadalu-nomad

# After install shipyard, create local cluster
-> shipyard run
[...]
-> eval $(shipyard env)
-> export job_dir="$(pwd)/kadalu"
```
- End of local cluster creation and nomad config


## Demo

### Pre-requisites
- You can configure vars mentioned in `cluster.vars` to reflect your external gluster details
- For convenience necessary vars are set from CLI while running the job

``` sh
-> export volname="sample-pool" gluster_hosts="10.x.x.x" gluster_volname="sample-vol" job_dir="${job_dir:-$(pwd)}"
# Make sure external gluster volume is started and quota is set
-> ssh $gluster_hosts "gluster volume info $gluster_volname | grep Status"
Status: Started

-> ssh $gluster_hosts "gluster volume quota $gluster_volname enable"
volume quota : success
```

### CSI Deployment

- Controller part
``` sh
-> nomad run -var="volname=$volname" -var="gluster_hosts=$gluster_hosts" -var="gluster_volname=$gluster_volname" $job_dir/controller.nomad
==> 2021-09-20T18:23:07+05:30: Monitoring evaluation "19317b74"
    2021-09-20T18:23:07+05:30: Evaluation triggered by job "kadalu-csi-controller"
==> 2021-09-20T18:23:08+05:30: Monitoring evaluation "19317b74"
    2021-09-20T18:23:08+05:30: Evaluation within deployment: "d9ee4dd7"
    2021-09-20T18:23:08+05:30: Allocation "d55e314d" created: node "4e105698", group "controller"
    2021-09-20T18:23:08+05:30: Evaluation status changed: "pending" -> "complete"
==> 2021-09-20T18:23:08+05:30: Evaluation "19317b74" finished with status "complete"
==> 2021-09-20T18:23:08+05:30: Monitoring deployment "d9ee4dd7"
  ✓ Deployment "d9ee4dd7" successful

    2021-09-20T18:23:28+05:30
    ID          = d9ee4dd7
    Job ID      = kadalu-csi-controller
    Job Version = 0
    Status      = successful
    Description = Deployment completed successfully

    Deployed
    Task Group  Desired  Placed  Healthy  Unhealthy  Progress Deadline
    controller  1        1       1        0          2021-09-20T13:03:27Z
```

- Nodeplugin part
``` sh
-> nomad run -var="volname=$volname" -var="gluster_hosts=$gluster_hosts" -var="gluster_volname=$gluster_volname" $job_dir/nodeplugin.nomad
==> 2021-09-20T18:23:53+05:30: Monitoring evaluation "bd4d95d1"
    2021-09-20T18:23:53+05:30: Evaluation triggered by job "kadalu-csi-nodeplugin"
==> 2021-09-20T18:23:54+05:30: Monitoring evaluation "bd4d95d1"
    2021-09-20T18:23:54+05:30: Allocation "4c05ab5a" created: node "4e105698", group "nodeplugin"
    2021-09-20T18:23:54+05:30: Evaluation status changed: "pending" -> "complete"
==> 2021-09-20T18:23:54+05:30: Evaluation "bd4d95d1" finished with status "complete"
```

- Check CSI plugin status
``` sh
-> nomad plugin status kadalu-csi
ID                   = kadalu-csi
Provider             = kadalu
Version              = 0.8.6
Controllers Healthy  = 1
Controllers Expected = 1
Nodes Healthy        = 1
Nodes Expected       = 1

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created    Modified
d55e314d  4e105698  controller  0        run      running  1m20s ago  1m ago
4c05ab5a  4e105698  nodeplugin  0        run      running  35s ago    20s ago
```

### Volume Ops

- We'll go through volume creation, attachment and deletion operations, basically, a typical volume life-cycle

``` sh
# Creation of nomad volume
-> sed -e "s/POOL/$volname/" -e "s/GHOST/$gluster_hosts/" -e "s/GVOL/$gluster_volname/" $job_dir/volume.hcl  | nomad volume create -
Created external volume csi-test with ID csi-test

# Attach the volume to a sample app
-> nomad run $job_dir/app.nomad
==> 2021-09-20T18:28:28+05:30: Monitoring evaluation "e6dd3129"
    2021-09-20T18:28:28+05:30: Evaluation triggered by job "sample-pv-check"
==> 2021-09-20T18:28:29+05:30: Monitoring evaluation "e6dd3129"
    2021-09-20T18:28:29+05:30: Evaluation within deployment: "814e328c"
    2021-09-20T18:28:29+05:30: Allocation "64745b25" created: node "4e105698", group "apps"
    2021-09-20T18:28:29+05:30: Evaluation status changed: "pending" -> "complete"
==> 2021-09-20T18:28:29+05:30: Evaluation "e6dd3129" finished with status "complete"
==> 2021-09-20T18:28:29+05:30: Monitoring deployment "814e328c"
  ✓ Deployment "814e328c" successful

    2021-09-20T18:28:58+05:30
    ID          = 814e328c
    Job ID      = sample-pv-check
    Job Version = 0
    Status      = successful
    Description = Deployment completed successfully

    Deployed
    Task Group  Desired  Placed  Healthy  Unhealthy  Progress Deadline
    apps        1        1       1        0          2021-09-20T13:08:56Z

# Allocation ID (64745b25) is from above o/p
-> export app=64745b25

# Verify CSI Volume is accessible or not
-> nomad alloc exec $app bash /kadalu/script.sh
This is a sample application
# df -h
Filesystem                               Size      Used Available Use% Mounted on
<gluster_hosts>:<gluster_volname>      181.2M         0    181.2M   0% /mnt/pv
# mount
Write/Read test on PV mount
Mon Sep 20 12:59:34 UTC 2021
SUCCESS

# Let's write some data on the volume
-> nomad alloc exec $app bash -c 'cd /mnt/pv; for i in {1..10}; do cat /dev/urandom | tr -dc [:space:][:print:] | head -c 1m > file$i; done;'

# Checksum of written data
-> nomad alloc exec $app bash -c 'ls /mnt/pv; find /mnt/pv -type f -exec md5sum {} + | cut -f1 -d" " | sort | md5sum'
file1   file2   file4   file6   file8
file10  file3   file5   file7   file9
6776dd355c0f2ba5a1781b9831e5c174  -

# We'll stop sample app and run again to check data persistence
-> nomad status
ID                         Type     Priority  Status   Submit Date
kadalu-csi-controller      service  50        running  2021-09-20T18:23:07+05:30
kadalu-csi-nodeplugin      system   50        running  2021-09-20T18:23:53+05:30
sample-pv-check            service  50        running  2021-09-20T18:28:28+05:30

-> nomad stop sample-pv-check
==> 2021-09-20T18:36:47+05:30: Monitoring evaluation "eecc0c00"
    2021-09-20T18:36:47+05:30: Evaluation triggered by job "sample-pv-check"
==> 2021-09-20T18:36:48+05:30: Monitoring evaluation "eecc0c00"
    2021-09-20T18:36:48+05:30: Evaluation within deployment: "814e328c"
    2021-09-20T18:36:48+05:30: Evaluation status changed: "pending" -> "complete"
==> 2021-09-20T18:36:48+05:30: Evaluation "eecc0c00" finished with status "complete"
==> 2021-09-20T18:36:48+05:30: Monitoring deployment "814e328c"
  ✓ Deployment "814e328c" successful

    2021-09-20T18:36:48+05:30
    ID          = 814e328c
    Job ID      = sample-pv-check
    Job Version = 0
    Status      = successful
    Description = Deployment completed successfully

    Deployed
    Task Group  Desired  Placed  Healthy  Unhealthy  Progress Deadline
    apps        1        1       1        0          2021-09-20T13:08:56Z

-> nomad run $job_dir/app.nomad
==> 2021-09-20T18:37:49+05:30: Monitoring evaluation "e04b4549"
    2021-09-20T18:37:49+05:30: Evaluation triggered by job "sample-pv-check"
==> 2021-09-20T18:37:50+05:30: Monitoring evaluation "e04b4549"
    2021-09-20T18:37:50+05:30: Evaluation within deployment: "66d246ee"
    2021-09-20T18:37:50+05:30: Allocation "526d5543" created: node "4e105698", group "apps"
    2021-09-20T18:37:50+05:30: Evaluation status changed: "pending" -> "complete"
==> 2021-09-20T18:37:50+05:30: Evaluation "e04b4549" finished with status "complete"
==> 2021-09-20T18:37:50+05:30: Monitoring deployment "66d246ee"
  ✓ Deployment "66d246ee" successful

    2021-09-20T18:38:10+05:30
    ID          = 66d246ee
    Job ID      = sample-pv-check
    Job Version = 2
    Status      = successful
    Description = Deployment completed successfully

    Deployed
    Task Group  Desired  Placed  Healthy  Unhealthy  Progress Deadline
    apps        1        1       1        0          2021-09-20T13:18:08Z

# Allocation ID is reset and md5sum is matched after stop and run of same job
-> export app=526d5543
-> nomad alloc exec $app bash -c 'ls /mnt/pv; find /mnt/pv -type f -exec md5sum {} + | cut -f1 -d" " | sort | md5sum'
file1   file10  file2   file3   file4   file5   file6   file7   file8   file9
6776dd355c0f2ba5a1781b9831e5c174  -

# Cleanup
-> nomad stop sample-pv-check
-> nomad volume delete csi-test
-> nomad stop kadalu-csi-nodeplugin
-> nomad stop kadalu-csi-controller

# Destroying local cluster
-> shipyard destroy
```

## Contact

- For any extra information/feature wrt Kadalu CSI, please raise an issue against kadalu [repo](https://github.com/kadalu/kadalu)
- For any extra information wrt local Nomad dev setup for CSI, please raise an issue against kadalu-nomad [repo](https://github.com/leelavg/kadalu-nomad)
- Based on ask/feature request, we may work on supporting internal gluster deployed and managed by Nomad itself (feature parity wrt current Kubernetes deployments)
- If this folder isn't updated frequently one can find updated jobs at kadalu nomad [folder](https://github.com/kadalu/kadalu/tree/devel/nomad)

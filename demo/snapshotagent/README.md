# Snapshot Agent Deployment

This directory demonstrates deploying the Nomad Snapshot Agent as a Nomad job using workload-associated ACL policies.

## Run the demo

Run a Nomad Enterprise cluster. Note that you cannot take snapshots against a `-dev` mode cluster. Create the `infra` namespace:

```hcl
$ nomad namespace apply infra
Successfully applied namespace "infra"!
```

Apply the workload-associated ACL policy to the snapshot job in that namespace:

```hcl
$ nomad acl policy apply -namespace infra -job snapshot-agent snapshot-agent ./policy-snapshot-agent.hcl
Successfully wrote "snapshot-agent" ACL policy!

$ nomad acl policy info snapshot-agent
Name        = snapshot-agent
Description = <none>
CreateIndex = 26
ModifyIndex = 26

Associated Workload
Namespace = infra
JobID     = snapshot-agent
Group     = <none>
Task      = <none>

Rules

operator {
  capabilities = ["snapshot-save", "license-read"]
}
```

Deploy the snapshot job:

```hcl
$ nomad job run ./snapshot-agent.nomad.hcl
Job registration successful
Approximate next launch time: 2026-05-01T19:30:00Z (20s from now)
```

Wait for the job to launch, then check the parent job to see the list of children:

```
$ nomad job status -namespace infra snapshot-agent
ID                   = snapshot-agent
Name                 = snapshot-agent
Submit Date          = 2026-05-01T15:29:40-04:00
Type                 = batch
Priority             = 50
Datacenters          = *
Namespace            = infra
Node Pool            = default
Status               = running
Periodic             = true
Parameterized        = false
Next Periodic Launch = 2026-05-01T19:35:00Z (5m0s from now)

Children Job Summary
Pending  Running  Dead
0        1        0

Previously Launched Jobs
ID                                  Status
snapshot-agent/periodic-1777663800  running
```

Find the allocation of the child job and check its logs:

```
$ nomad job allocs -namespace infra snapshot-agent/periodic-1777663800
ID        Node ID   Task Group  Version  Desired  Status    Created   Modified
0918feb3  78b8f2a8  group       0        run      complete  3m4s ago  3m3s ago

$ nomad alloc logs -namespace infra 0918feb3
Nomad snapshot agent running!
        Interval: "0s"
          Retain: 10647988397472
           Stale: false
   Local Scratch: /snapshots
            Mode: one-shot
Snapshot Storage: Local -> Path: "/snapshots"

Log data will now stream in as it occurs:

2026-05-01T19:30:00.223Z [INFO]  snapshotagent.license: automatic snapshot is licensed
2026-05-01T19:30:00.229Z [INFO]  snapshotagent.snapshot: Saved snapshot: id=1777663800228121705
```

Note that this snapshot job runs every 5 minutes and saves snapshots to a local directory. You'll want to configure the snapshot job to run less frequently, and save them to a remote location. See the [`nomad operator snapshot agent`](https://developer.hashicorp.com/nomad/commands/operator/snapshot/agent) command documentation.

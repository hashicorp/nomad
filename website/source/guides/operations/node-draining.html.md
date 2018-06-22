---
layout: "guides"
page_title: "Workload Migration"
sidebar_current: "guides-operations-decommissioning-nodes"
description: |-
  Workload migration is a normal part of cluster operations for a variety of
  reasons: server maintenance, operating system upgrades, etc. Nomad offers a
  number of parameters for controlling how running jobs are migrated off of
  draining nodes.
---

# Workload Migration

Migrating workloads and decommissioning nodes are a normal part of cluster 
operations for a variety of reasons: server maintenance, operating system 
upgrades, etc. Nomad offers a number of parameters for controlling how running 
jobs are migrated off of draining nodes.

## Configuring How Jobs are Migrated

In Nomad 0.8 a [`migrate`][migrate] stanza was added to jobs to allow control
over how allocations for a job are migrated off of a draining node. Below is an
example job that runs a web service and has a Consul health check:

```hcl
job "webapp" {
  datacenters = ["dc1"]

  migrate {
    max_parallel = 2
    health_check = "checks"
    min_healthy_time = "15s"
    healthy_deadline = "5m"
  }

  group "webapp" {
    count = 9

    task "webapp" {
      driver = "docker"
      config {
        image = "hashicorp/http-echo:0.2.3"
        args  = ["-text", "ok"]
        port_map {
          http = 5678
        }
      }

      resources {
        network {
          mbits = 10
          port "http" {}
        }
      }

      service {
        name = "webapp"
        port = "http"
        check {
          name = "http-ok"
          type = "http"
          path = "/"
          interval = "10s"
          timeout  = "2s"
        }
      }
    }
  }
}
```

The above `migrate` stanza ensures only 2 allocations are stopped at a time to
migrate during node drains. Even if multiple nodes running allocations for this
job were draining at the same time, only 2 allocations would be migrated at a
time.

When the job is run it may be placed on multiple nodes. In the following
example the 9 `webapp` allocations are spread across 2 nodes: 

```text
$ nomad run webapp.nomad
==> Monitoring evaluation "5129bc74"
    Evaluation triggered by job "webapp"
    Allocation "5b4d6db5" created: node "46f1c6c4", group "webapp"
    Allocation "670a715f" created: node "f7476465", group "webapp"
    Allocation "78b6b393" created: node "46f1c6c4", group "webapp"
    Allocation "85743ff5" created: node "f7476465", group "webapp"
    Allocation "edf71a5d" created: node "f7476465", group "webapp"
    Allocation "56f770c0" created: node "46f1c6c4", group "webapp"
    Allocation "9a51a484" created: node "46f1c6c4", group "webapp"
    Allocation "f6f6e64c" created: node "f7476465", group "webapp"
    Allocation "fefe81d0" created: node "f7476465", group "webapp"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "5129bc74" finished with status "complete"
```

If one those nodes needed to be decommissioned, perhaps because of a hardware
issue, then an operator would issue node drain to migrate the allocations off:

```text
$ nomad node drain -enable -yes 46f1
2018-04-11T23:41:56Z: Ctrl-C to stop monitoring: will not cancel the node drain
2018-04-11T23:41:56Z: Node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" drain strategy set
2018-04-11T23:41:57Z: Alloc "5b4d6db5-3fcb-eb7d-0415-23eefcd78b6a" marked for migration
2018-04-11T23:41:57Z: Alloc "56f770c0-f8aa-4565-086d-01faa977f82d" marked for migration
2018-04-11T23:41:57Z: Alloc "56f770c0-f8aa-4565-086d-01faa977f82d" draining
2018-04-11T23:41:57Z: Alloc "5b4d6db5-3fcb-eb7d-0415-23eefcd78b6a" draining
2018-04-11T23:42:03Z: Alloc "56f770c0-f8aa-4565-086d-01faa977f82d" status running -> complete
2018-04-11T23:42:03Z: Alloc "5b4d6db5-3fcb-eb7d-0415-23eefcd78b6a" status running -> complete
2018-04-11T23:42:22Z: Alloc "78b6b393-d29c-d8f8-e8e8-28931c0013ee" marked for migration
2018-04-11T23:42:22Z: Alloc "78b6b393-d29c-d8f8-e8e8-28931c0013ee" draining
2018-04-11T23:42:27Z: Alloc "78b6b393-d29c-d8f8-e8e8-28931c0013ee" status running -> complete
2018-04-11T23:42:29Z: Alloc "9a51a484-8c43-aa4e-d60a-46cfd1450780" marked for migration
2018-04-11T23:42:29Z: Alloc "9a51a484-8c43-aa4e-d60a-46cfd1450780" draining
2018-04-11T23:42:29Z: Node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" has marked all allocations for migration
2018-04-11T23:42:34Z: Alloc "9a51a484-8c43-aa4e-d60a-46cfd1450780" status running -> complete
2018-04-11T23:42:34Z: All allocations on node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" have stopped.
```

There are a couple important events to notice in the output. First, only 2
allocations are migrated initially:

```
2018-04-11T23:41:57Z: Alloc "5b4d6db5-3fcb-eb7d-0415-23eefcd78b6a" marked for migration
2018-04-11T23:41:57Z: Alloc "56f770c0-f8aa-4565-086d-01faa977f82d" marked for migration
```

This is because `max_parallel = 2` in the job specification. The next
allocation on the draining node waits to be migrated:

```
2018-04-11T23:42:22Z: Alloc "78b6b393-d29c-d8f8-e8e8-28931c0013ee" marked for migration
```

Note that this occurs 25 seconds after the initial migrations. The 25 second
delay is because a replacement allocation took 10 seconds to become healthy and
then the `min_healthy_time = "15s"` meant node draining waited an additional 15
seconds. If the replacement allocation had failed within that time the node
drain would not have continued until a replacement could be successfully made.

### Scheduling Eligibility

Now that the example drain has finished we can inspect the state of the drained
node:

```text
$ nomad node status
ID        DC   Name     Class   Drain  Eligibility  Status
f7476465  dc1  nomad-1  <none>  false  eligible     ready
96b52ad8  dc1  nomad-2  <none>  false  eligible     ready
46f1c6c4  dc1  nomad-3  <none>  false  ineligible   ready
```

While node `46f1c6c4` has `Drain = false`, notice that its `Eligibility =
ineligible`. Node scheduling eligibility is a new field in Nomad 0.8. When a
node is ineligible for scheduling the scheduler will not consider it for new
placements.

While draining, a node will always be ineligible for scheduling. Once draining
completes it will remain ineligible to prevent refilling a newly drained node.

However, by default canceling a drain with the `-disable` option will reset a
node to be eligible for scheduling. To cancel a drain and preserving the node's
ineligible status use the `-keep-ineligible` option.

Scheduling eligibility can be toggled independently of node drains by using the
[`nomad node eligibility`][eligibility] command:

```text
$ nomad node eligibility -disable 46f1
Node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" scheduling eligibility set: ineligible for scheduling
```

### Node Drain Deadline

Sometimes a drain is unable to proceed and complete normally. This could be
caused by not enough capacity existing in the cluster to replace the drained
allocations or by replacement allocations failing to start successfully in a
timely fashion.

Operators may specify a deadline when enabling a node drain to prevent drains
from not finishing. Once the deadline is reached, all remaining allocations on
the node are stopped regardless of `migrate` stanza parameters.

The default deadline is 1 hour and may be changed with the
[`-deadline`][deadline] command line option. The [`-force`][force] option is an
instant deadline: all allocations are immediately stopped. The
[`-no-deadline`][no-deadline] option disables the deadline so a drain may
continue indefinitely.

Like all other drain parameters, a drain's deadline can be updated by making
subsequent `nomad node drain ...` calls with updated values.

## Node Drains and Non-Service Jobs

So far we have only seen how draining works with service jobs. Both batch and
system jobs are have different behaviors during node drains.

### Draining Batch Jobs

Node drains only migrate batch jobs once the drain's deadline has been reached.
For node drains without a deadline the drain will not complete until all batch
jobs on the node have completed (or failed).

The goal of this behavior is to avoid losing progress a batch job has made by
forcing it to exit early.

### Keeping System Jobs Running

Node drains only stop system jobs once all other allocations have exited. This
way if a node is running a log shipping daemon or metrics collector as a system
job, it will continue to run as long as there are other allocations running.

The [`-ignore-system`][ignore-system] option leaves system jobs running even
after all other allocations have exited. This is useful when system jobs are
used to monitor Nomad or the node itself.

## Draining Multiple Nodes

A common operation is to decommission an entire class of nodes at once. Prior
to Nomad 0.7 this was a problematic operation as the first node to begin
draining may migrate all of their allocations to the next node about to be
drained. In pathological cases this could repeat on each node to be drained and
cause allocations to be rescheduled repeatedly.

As of Nomad 0.8 an operator can avoid this churn by marking nodes ineligible
for scheduling before draining them using the [`nomad node
eligibility`][eligibility] command:

```text
$ nomad node eligibility -disable 46f1
Node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" scheduling eligibility set: ineligible for scheduling

$ nomad node eligibility -disable 96b5
Node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" scheduling eligibility set: ineligible for scheduling

$ nomad node status
ID        DC   Name     Class   Drain  Eligibility  Status
f7476465  dc1  nomad-1  <none>  false  eligible     ready
46f1c6c4  dc1  nomad-2  <none>  false  ineligible   ready
96b52ad8  dc1  nomad-3  <none>  false  ineligible   ready
```

Now that both `nomad-2` and `nomad-3` are ineligible for scheduling, they can
be drained without risking placing allocations on an _about-to-be-drained_
node.

Toggling scheduling eligibility can be done totally independently of draining.
For example when an operator wants to inspect the allocations currently running
on a node without risking new allocations being scheduled and changing the
node's state:

```text
$ nomad node eligibility -self -disable
Node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" scheduling eligibility set: ineligible for scheduling

$ # ...inspect node state...

$ nomad node eligibility -self -enable
Node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" scheduling eligibility set: eligible for scheduling
```

### Example: Migrating Datacenters

A more complete example of draining multiple nodes would be when migrating from
an old datacenter (`dc1`) to a new datacenter (`dc2`):

```text
$ nomad node status -allocs
ID        DC   Name     Class   Drain  Eligibility  Status  Running Allocs
f7476465  dc1  nomad-1  <none>  false  eligible     ready   4
46f1c6c4  dc1  nomad-2  <none>  false  eligible     ready   1
96b52ad8  dc1  nomad-3  <none>  false  eligible     ready   4
168bdd03  dc2  nomad-4  <none>  false  eligible     ready   0
9ccb3306  dc2  nomad-5  <none>  false  eligible     ready   0
7a7f9a37  dc2  nomad-6  <none>  false  eligible     ready   0
```

Before migrating ensure that all jobs in `dc1` have `datacenters = ["dc1",
"dc2"]`.  Then before draining, mark all nodes in `dc1` as ineligible for
scheduling. Shell scripting can help automate manipulating multiple nodes at
once:

```text
$ nomad node status | awk '{ print $2 " " $1 }' | grep ^dc1 | awk '{ system("nomad node eligibility -disable "$2) }'
Node "f7476465-4d6e-c0de-26d0-e383c49be941" scheduling eligibility set: ineligible for scheduling
Node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" scheduling eligibility set: ineligible for scheduling
Node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" scheduling eligibility set: ineligible for scheduling

$ nomad node status
ID        DC   Name     Class   Drain  Eligibility  Status
f7476465  dc1  nomad-1  <none>  false  ineligible   ready
46f1c6c4  dc1  nomad-2  <none>  false  ineligible   ready
96b52ad8  dc1  nomad-3  <none>  false  ineligible   ready
168bdd03  dc2  nomad-4  <none>  false  eligible     ready
9ccb3306  dc2  nomad-5  <none>  false  eligible     ready
7a7f9a37  dc2  nomad-6  <none>  false  eligible     ready
```

Then drain each node in `dc1`. For this example we will only monitor the final
node that is draining. Watching `nomad node status -allocs` is also a good way
to monitor the status of drains.

```text
$ nomad node drain -enable -yes -detach f7476465
Node "f7476465-4d6e-c0de-26d0-e383c49be941" drain strategy set

$ nomad node drain -enable -yes -detach 46f1c6c4
Node "46f1c6c4-a0e5-21f6-fd5c-d76c3d84e806" drain strategy set

$ nomad node drain -enable -yes 9ccb3306
2018-04-12T22:08:00Z: Ctrl-C to stop monitoring: will not cancel the node drain
2018-04-12T22:08:00Z: Node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" drain strategy set
2018-04-12T22:08:15Z: Alloc "392ee2ec-d517-c170-e7b1-d93b2d44642c" marked for migration
2018-04-12T22:08:16Z: Alloc "392ee2ec-d517-c170-e7b1-d93b2d44642c" draining
2018-04-12T22:08:17Z: Alloc "6a833b3b-c062-1f5e-8dc2-8b6af18a5b94" marked for migration
2018-04-12T22:08:17Z: Alloc "6a833b3b-c062-1f5e-8dc2-8b6af18a5b94" draining
2018-04-12T22:08:21Z: Alloc "392ee2ec-d517-c170-e7b1-d93b2d44642c" status running -> complete
2018-04-12T22:08:22Z: Alloc "6a833b3b-c062-1f5e-8dc2-8b6af18a5b94" status running -> complete
2018-04-12T22:09:08Z: Alloc "d572d7a3-024b-fcb7-128b-1932a49c8d79" marked for migration
2018-04-12T22:09:09Z: Alloc "d572d7a3-024b-fcb7-128b-1932a49c8d79" draining
2018-04-12T22:09:14Z: Alloc "d572d7a3-024b-fcb7-128b-1932a49c8d79" status running -> complete
2018-04-12T22:09:33Z: Alloc "f3f24277-4435-56a3-7ee1-1b1eff5e3aa1" marked for migration
2018-04-12T22:09:33Z: Alloc "f3f24277-4435-56a3-7ee1-1b1eff5e3aa1" draining
2018-04-12T22:09:33Z: Node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" has marked all allocations for migration
2018-04-12T22:09:39Z: Alloc "f3f24277-4435-56a3-7ee1-1b1eff5e3aa1" status running -> complete
2018-04-12T22:09:39Z: All allocations on node "96b52ad8-e9ad-1084-c14f-0e11f10772e4" have stopped.
```

Note that there was a 15 second delay between node `96b52ad8` starting to drain
and having its first allocation migrated. The delay was due to 2 other
allocations for the same job already being migrated from the other nodes. Once
at least 8 out of the 9 allocations are running for the job, another allocation
could begin draining.

The final node drain command did not exit until 6 seconds after the `drain
complete` message because the command line tool blocks until all allocations on
the node have stopped. This allows operators to script shutting down a node
once a drain command exits and know all services have already exited.

[deadline]: /docs/commands/node/drain.html#deadline
[eligibility]: /docs/commands/node/eligibility.html
[force]: /docs/commands/node/drain.html#force
[ignore-system]: /docs/commands/node/drain.html#ignore-system
[migrate]: /docs/job-specification/migrate.html
[no-deadline]: /docs/commands/node/drain.html#no-deadline

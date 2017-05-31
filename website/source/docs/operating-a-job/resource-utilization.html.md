---
layout: "docs"
page_title: "Resource Utilization - Operating a Job"
sidebar_current: "docs-operating-a-job-resource-utilization"
description: |-
  Nomad supports reporting detailed job statistics and resource utilization
  metrics for most task drivers. This section describes the ways to inspect a
  job's resource consumption and utilization.
---

# Resource Utilization

Understanding the resource utilization of an application is important, and Nomad
supports reporting detailed statistics in many of its drivers. The main
interface for seeing resource utilization is the `alloc-status` command with the
`-stats` flag.

This section will utilize the job named "docs" from the [previous
sections](/docs/operating-a-job/submitting-jobs.html), but these operations
and command largely apply to all jobs in Nomad.

As a reminder, here is the output of the run command from the previous example:

```text
$ nomad run docs.nomad
==> Monitoring evaluation "42d788a3"
    Evaluation triggered by job "docs"
    Allocation "04d9627d" created: node "a1f934c9", group "example"
    Allocation "e7b8d4f5" created: node "012ea79b", group "example"
    Allocation "5cbf23a1" modified: node "1e1aa1e0", group "example"
    Evaluation status changed: "pending" -> "complete"
==> Evaluation "42d788a3" finished with status "complete"
```

To see the detailed usage statistics, we can issue the command:

```shell
$ nomad alloc-status -stats 04d9627d
```

And here is some sample output:

```text
$ nomad alloc-status c3e0
ID            = 04d9627d
Eval ID       = 42d788a3
Name          = docs.example[2]
Node ID       = a1f934c9
Job ID        = docs
Client Status = running

Task "server" is "running"
Task Resources
CPU        Memory          Disk     IOPS  Addresses
75/100 MHz  784 KiB/10 MiB  300 MiB  0     http: 10.1.1.196:5678

Memory Stats
Cache   Max Usage  RSS      Swap
56 KiB  1.3 MiB    784 KiB  0 B

CPU Stats
Percent  Throttled Periods  Throttled Time
0.00%    0                  0

Recent Events:
Time         Type      Description
<timestamp>  Started   Task started by client
<timestamp>  Received  Task received by client
```

Here we can see that we are near the limit of our configured CPU but we have
plenty of memory headroom. We can use this information to alter our job's
resources to better reflect is actually needs:

```hcl
resource {
  cpu    = 200
  memory = 10
}
```

Adjusting resources is very important for a variety of reasons:

* Ensuring your application does not get OOM killed if it hits its memory limit.
* Ensuring the application performs well by ensuring it has some CPU allowance.
* Optimizing cluster density by reserving what you need and not over-allocating.

While single point in time resource usage measurements are useful, it is often
more useful to graph resource usage over time to better understand and estimate
resource usage. Nomad supports outputting resource data to statsite and statsd
and is the recommended way of monitoring resources. For more information about
outputting telemetry see the [telemetry
documentation](/docs/agent/telemetry.html).

For more advanced use cases, the resource usage data is also accessible via the
client's HTTP API. See the documentation of the Client's [allocation HTTP
API](/api/client.html).

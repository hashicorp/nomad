---
layout: "guides"
page_title: "Inspecting State - Operating a Job"
sidebar_current: "guides-operating-a-job-inspecting-state"
description: |-
  Nomad exposes a number of tools and techniques for inspecting a running job.
  This is helpful in ensuring the job started successfully. Additionally, it
  can inform us of any errors that occurred while starting the job.
---

# Inspecting State

A successful job submission is not an indication of a successfully-running job.
This is the nature of a highly-optimistic scheduler. A successful job submission
means the server was able to issue the proper scheduling commands. It does not
indicate the job is actually running. To verify the job is running, we need to
inspect its state.

This section will utilize the job named "docs" from the [previous
sections](/guides/operating-a-job/submitting-jobs.html), but these operations
and command largely apply to all jobs in Nomad.

## Job Status

After a job is submitted, you can query the status of that job using the job
status command:

```text
$ nomad job status
ID    Type     Priority  Status
docs  service  50        running
```

At a high level, we can see that our job is currently running, but what does
"running" actually mean. By supplying the name of a job to the job status
command, we can ask Nomad for more detailed job information:

```text
$ nomad job status docs
ID          = docs
Name        = docs
Type        = service
Priority    = 50
Datacenters = dc1
Status      = running
Periodic    = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
example     0       0         3        0       0         0

Allocations
ID        Eval ID   Node ID   Task Group  Desired  Status    Created At
04d9627d  42d788a3  a1f934c9  example     run      running   <timestamp>
e7b8d4f5  42d788a3  012ea79b  example     run      running   <timestamp>
5cbf23a1  42d788a3  1e1aa1e0  example     run      running   <timestamp>
```

Here we can see that there are three instances of this task running, each with
its own allocation. For more information on the `status` command, please see the
[CLI documentation for <tt>status</tt>](/docs/commands/status.html).

## Evaluation Status

You can think of an evaluation as a submission to the scheduler. An example
below shows status output for a job where some allocations were placed
successfully, but did not have enough resources to place all of the desired
allocations.

If we issue the status command with the `-evals` flag, we could see there is an
outstanding evaluation for this hypothetical job:

```text
$ nomad job status -evals docs
ID          = docs
Name        = docs
Type        = service
Priority    = 50
Datacenters = dc1
Status      = running
Periodic    = false

Evaluations
ID        Priority  Triggered By  Status    Placement Failures
5744eb15  50        job-register  blocked   N/A - In Progress
8e38e6cf  50        job-register  complete  true

Placement Failure
Task Group "example":
  * Resources exhausted on 1 nodes
  * Dimension "cpu" exhausted on 1 nodes

Allocations
ID        Eval ID   Node ID   Task Group  Desired  Status   Created At
12681940  8e38e6cf  4beef22f  example       run      running  <timestamp>
395c5882  8e38e6cf  4beef22f  example       run      running  <timestamp>
4d7c6f84  8e38e6cf  4beef22f  example       run      running  <timestamp>
843b07b8  8e38e6cf  4beef22f  example       run      running  <timestamp>
a8bc6d3e  8e38e6cf  4beef22f  example       run      running  <timestamp>
b0beb907  8e38e6cf  4beef22f  example       run      running  <timestamp>
da21c1fd  8e38e6cf  4beef22f  example       run      running  <timestamp>
```

In the above example we see that the job has a "blocked" evaluation that is in
progress. When Nomad can not place all the desired allocations, it creates a
blocked evaluation that waits for more resources to become available.

The `eval status` command enables us to examine any evaluation in more detail.
For the most part this should never be necessary but can be useful to see why
all of a job's allocations were not placed. For example if we run it on the job
named docs, which had a placement failure according to the above output, we
might see:

```text
$ nomad eval status 8e38e6cf
ID                 = 8e38e6cf
Status             = complete
Status Description = complete
Type               = service
TriggeredBy        = job-register
Job ID             = docs
Priority           = 50
Placement Failures = true

Failed Placements
Task Group "example" (failed to place 3 allocations):
  * Resources exhausted on 1 nodes
  * Dimension "cpu" exhausted on 1 nodes

Evaluation "5744eb15" waiting for additional capacity to place remainder
```

For more information on the `eval status` command, please see the [CLI documentation for <tt>eval status</tt>](/docs/commands/eval-status.html).

## Allocation Status

You can think of an allocation as an instruction to schedule. Just like an
application or service, an allocation has logs and state. The `alloc status`
command gives us the most recent events that occurred for a task, its resource
usage, port allocations and more:

```text
$ nomad alloc status 04d9627d
ID            = 04d9627d
Eval ID       = 42d788a3
Name          = docs.example[2]
Node ID       = a1f934c9
Job ID        = docs
Client Status = running

Task "server" is "running"
Task Resources
CPU        Memory          Disk     IOPS  Addresses
0/100 MHz  728 KiB/10 MiB  300 MiB  0     http: 10.1.1.196:5678

Recent Events:
Time                   Type      Description
10/09/16 00:36:06 UTC  Started   Task started by client
10/09/16 00:36:05 UTC  Received  Task received by client
```

The `alloc status` command is a good starting to point for debugging an
application that did not start. Hypothetically assume a user meant to start a
Docker container named "redis:2.8", but accidentally put a comma instead of a
period, typing "redis:2,8".

When the job is executed, it produces a failed allocation. The `alloc status`
command will give us the reason why:

```text
$ nomad alloc status 04d9627d
# ...

Recent Events:
Time                   Type            Description
06/28/16 15:50:22 UTC  Not Restarting  Error was unrecoverable
06/28/16 15:50:22 UTC  Driver Failure  failed to create image: Failed to pull `redis:2,8`: API error (500): invalid tag format
06/28/16 15:50:22 UTC  Received        Task received by client
```

Unfortunately not all failures are as easily debuggable. If the `alloc status`
command shows many restarts, there is likely an application-level issue during
start up. For example:

```text
$ nomad alloc status 04d9627d
# ...

Recent Events:
Time                   Type        Description
06/28/16 15:56:16 UTC  Restarting  Task restarting in 5.178426031s
06/28/16 15:56:16 UTC  Terminated  Exit Code: 1, Exit Message: "Docker container exited with non-zero exit code: 1"
06/28/16 15:56:16 UTC  Started     Task started by client
06/28/16 15:56:00 UTC  Restarting  Task restarting in 5.00123931s
06/28/16 15:56:00 UTC  Terminated  Exit Code: 1, Exit Message: "Docker container exited with non-zero exit code: 1"
06/28/16 15:55:59 UTC  Started     Task started by client
06/28/16 15:55:48 UTC  Received    Task received by client
```

To debug these failures, we will need to utilize the "logs" command, which is
discussed in the [accessing logs](/guides/operating-a-job/accessing-logs.html)
section of this documentation.

For more information on the `alloc status` command, please see the [CLI
documentation for <tt>alloc status</tt>](/docs/commands/alloc/status.html).

---
layout: "docs"
page_title: "Restart Stanza - Operating a Job"
sidebar_current: "docs-operating-a-job-failure-handling-strategies-local-restarts"
description: |-
  Nomad can restart a task on the node it is running on to recover from
  failures. Task restarts can be configured to be limited by number of attempts within
  a specific interval.
---

# Restart Stanza

To enable restarting a failed task on the node it is running on, the task group can be annotated
with configurable options using the [`restart` stanza][restart]. Nomad will restart the failed task
upto `attempts` times within a provided `interval`. Operators can also choose whether to
keep attempting restarts on the same node, or to fail the task so that it can be rescheduled
on another node, via the `mode` parameter.

We recommend setting mode to `fail` in the restart stanza to allow rescheduling the task on another node.


## Example
The following CLI example shows job status and allocation status for a failed task that is being restarted by Nomad.
Allocations are in the `pending` state while restarts are attempted. The `Recent Events` section in the CLI
shows ongoing restart attempts.

```text
$nomad job status demo
ID            = demo
Name          = demo
Submit Date   = 2018-04-12T14:37:18-05:00
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = running
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
demo        0       3         0        0       0         0

Allocations
ID        Node ID   Task Group  Version  Desired  Status   Created  Modified
ce5bf1d1  8a184f31  demo        0        run      pending  27s ago  5s ago
d5dee7c8  8a184f31  demo        0        run      pending  27s ago  5s ago
ed815997  8a184f31  demo        0        run      pending  27s ago  5s ago
```

```text
$nomad alloc-status ce5b
ID                  = ce5bf1d1
Eval ID             = 05681b90
Name                = demo.demo[1]
Node ID             = 8a184f31
Job ID              = demo
Job Version         = 0
Client Status       = pending
Client Description  = <none>
Desired Status      = run
Desired Description = <none>
Created             = 31s ago
Modified            = 9s ago

Task "demo" is "pending"
Task Resources
CPU      Memory   Disk     IOPS  Addresses
100 MHz  300 MiB  300 MiB  0

Task Events:
Started At     = 2018-04-12T19:37:40Z
Finished At    = N/A
Total Restarts = 3
Last Restart   = 2018-04-12T14:37:40-05:00

Recent Events:
Time                       Type        Description
2018-04-12T14:37:40-05:00  Restarting  Task restarting in 11.686056069s
2018-04-12T14:37:40-05:00  Terminated  Exit Code: 127
2018-04-12T14:37:40-05:00  Started     Task started by client
2018-04-12T14:37:29-05:00  Restarting  Task restarting in 10.97348449s
2018-04-12T14:37:29-05:00  Terminated  Exit Code: 127
2018-04-12T14:37:29-05:00  Started     Task started by client
2018-04-12T14:37:19-05:00  Restarting  Task restarting in 10.619985509s
2018-04-12T14:37:19-05:00  Terminated  Exit Code: 127
2018-04-12T14:37:19-05:00  Started     Task started by client
2018-04-12T14:37:19-05:00  Task Setup  Building Task Directory
```


[restart]: /docs/job-specification/restart.html "Nomad restart Stanza"

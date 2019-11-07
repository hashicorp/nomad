---
layout: "guides"
page_title: "Restart Stanza - Operating a Job"
sidebar_current: "guides-operating-a-job-failure-handling-strategies-local-restarts"
description: |-
  Nomad can restart a task on the node it is running on to recover from
  failures. Task restarts can be configured to be limited by number of attempts within
  a specific interval.
---

# Restart Stanza

To enable restarting a failed task on the node it is running on, the task group can be annotated
with configurable options using the [`restart` stanza][restart]. Nomad will restart the failed task
up to `attempts` times within a provided `interval`. Operators can also choose whether to
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

In the following example, the allocation `ce5bf1d1` is restarted by Nomad approximately
every ten seconds, with a small random jitter. It eventually reaches its limit of three attempts and
transitions into a `failed` state, after which it becomes eligible for [rescheduling][rescheduling].

```text
$nomad alloc-status ce5bf1d1
ID                     = ce5bf1d1
Eval ID                = 64e45d11
Name                   = demo.demo[1]
Node ID                = a0ccdd8b
Job ID                 = demo
Job Version            = 0
Client Status          = failed
Client Description     = <none>
Desired Status         = run
Desired Description    = <none>
Created                = 56s ago
Modified               = 22s ago

Task "demo" is "dead"
Task Resources
CPU      Memory   Disk     Addresses
100 MHz  300 MiB  300 MiB

Task Events:
Started At     = 2018-04-12T22:29:08Z
Finished At    = 2018-04-12T22:29:08Z
Total Restarts = 3
Last Restart   = 2018-04-12T17:28:57-05:00

Recent Events:
Time                       Type            Description
2018-04-12T17:29:08-05:00  Not Restarting  Exceeded allowed attempts 3 in interval 5m0s and mode is "fail"
2018-04-12T17:29:08-05:00  Terminated      Exit Code: 127
2018-04-12T17:29:08-05:00  Started         Task started by client
2018-04-12T17:28:57-05:00  Restarting      Task restarting in 10.364602876s
2018-04-12T17:28:57-05:00  Terminated      Exit Code: 127
2018-04-12T17:28:57-05:00  Started         Task started by client
2018-04-12T17:28:47-05:00  Restarting      Task restarting in 10.666963769s
2018-04-12T17:28:47-05:00  Terminated      Exit Code: 127
2018-04-12T17:28:47-05:00  Started         Task started by client
2018-04-12T17:28:35-05:00  Restarting      Task restarting in 11.777324721s
```


[restart]: /docs/job-specification/restart.html "Nomad restart Stanza"
[rescheduling]: /docs/job-specification/reschedule.html "Nomad restart Stanza"

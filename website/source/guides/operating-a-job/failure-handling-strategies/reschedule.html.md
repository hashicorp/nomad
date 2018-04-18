---
layout: "guides"
page_title: "Reschedule Stanza - Operating a Job"
sidebar_current: "guides-operating-a-job-failure-handling-strategies-reschedule"
description: |-
  Nomad can reschedule failing tasks after any local restart attempts have been
  exhausted. This is useful to recover from failures stemming from problems in the node
  running the task.
---

# Reschedule Stanza

Tasks can sometimes fail due to network, CPU or memory issues on the node running the task. In such situations,
Nomad can reschedule the task on another node. The [`reschedule` stanza][reschedule] can be used to configure how
Nomad should try placing failed tasks on another node in the cluster. Reschedule attempts have a delay between
each attempt, and the delay can be configured to increase between each rescheduling attempt according to a configurable
`delay_function`. See the [`reschedule` stanza][reschedule] for more information.

Service jobs are configured by default to have unlimited reschedule attempts. We recommend using the reschedule
stanza to ensure that failed tasks are automatically reattempted on another node without needing operator intervention.

# Example
The following CLI example shows job and allocation statuses for a task being rescheduled by Nomad.
The CLI shows the number of previous attempts if there is a limit on the number of reschedule attempts.
The CLI also shows when the next reschedule will be attempted.

```text
$nomad job status demo
ID            = demo
Name          = demo
Submit Date   = 2018-04-12T15:48:37-05:00
Type          = service
Priority      = 50
Datacenters   = dc1
Status        = pending
Periodic      = false
Parameterized = false

Summary
Task Group  Queued  Starting  Running  Failed  Complete  Lost
demo        0       0         0        2       0         0

Future Rescheduling Attempts
Task Group  Eval ID   Eval Time
demo        ee3de93f  5s from now

Allocations
ID        Node ID   Task Group  Version  Desired  Status  Created  Modified
39d7823d  f2c2eaa6  demo        0        run      failed  5s ago   5s ago
fafb011b  f2c2eaa6  demo        0        run      failed  11s ago  10s ago

```

```text
$nomad alloc status 3d0b
ID                     = 3d0bbdb1
Eval ID                = 79b846a9
Name                   = demo.demo[0]
Node ID                = 8a184f31
Job ID                 = demo
Job Version            = 0
Client Status          = failed
Client Description     = <none>
Desired Status         = run
Desired Description    = <none>
Created                = 15s ago
Modified               = 15s ago
Reschedule Attempts    = 3/5
Reschedule Eligibility = 25s from now

Task "demo" is "dead"
Task Resources
CPU      Memory   Disk     IOPS  Addresses
100 MHz  300 MiB  300 MiB  0     p1: 127.0.0.1:27646

Task Events:
Started At     = 2018-04-12T20:44:25Z
Finished At    = 2018-04-12T20:44:25Z
Total Restarts = 0
Last Restart   = N/A

Recent Events:
Time                       Type            Description
2018-04-12T15:44:25-05:00  Not Restarting  Policy allows no restarts
2018-04-12T15:44:25-05:00  Terminated      Exit Code: 127
2018-04-12T15:44:25-05:00  Started         Task started by client
2018-04-12T15:44:25-05:00  Task Setup      Building Task Directory
2018-04-12T15:44:25-05:00  Received        Task received by client

```

[reschedule]: /docs/job-specification/reschedule.html "Nomad reschedule Stanza"
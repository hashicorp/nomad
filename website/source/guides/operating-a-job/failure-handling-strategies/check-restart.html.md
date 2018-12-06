---
layout: "guides"
page_title: "Check Restart Stanza - Operating a Job"
sidebar_current: "guides-operating-a-job-failure-handling-strategies-check-restart"
description: |-
  Nomad can restart tasks if they have a failing health check based on
  configuration specified in the `check_restart` stanza. Restarts are done locally on the node
  running the task based on their `restart` policy.
---

# Check Restart Stanza

The [`check_restart` stanza][check restart] instructs Nomad when to restart tasks with unhealthy service checks.
When a health check in Consul has been unhealthy for the limit specified in a check_restart stanza,
it is restarted according to the task group's restart policy.

The `limit ` field is used to specify the number of times a failing healthcheck is seen before local restarts are attempted.
Operators can also specify a `grace` duration to wait after a task restarts before checking its health.

We recommend configuring the check restart on services if its likely that a restart would resolve the failure. This
is applicable in cases like temporary memory issues on the service.

# Example

The following `check_restart` stanza waits for two consecutive health check failures with a
grace period and considers both `critical` and `warning` statuses as failures

```text
check_restart {
  limit           = 2
  grace           = "10s"
  ignore_warnings = false
}
```

The following CLI example output shows healthcheck failures triggering restarts until its
restart limit is reached.

```
$nomad alloc status e1b43128-2a0a-6aa3-c375-c7e8a7c48690
ID                   = e1b43128
Eval ID              = 249cbfe9
Name                 = demo.demo[0]
Node ID              = 221e998e
Job ID               = demo
Job Version          = 0
Client Status        = failed
Client Description   = <none>
Desired Status       = run
Desired Description  = <none>
Created              = 2m59s ago
Modified             = 39s ago

Task "test" is "dead"
Task Resources
CPU      Memory   Disk     Addresses
100 MHz  300 MiB  300 MiB  p1: 127.0.0.1:28422

Task Events:
Started At     = 2018-04-12T22:50:32Z
Finished At    = 2018-04-12T22:50:54Z
Total Restarts = 3
Last Restart   = 2018-04-12T17:50:15-05:00

Recent Events:
Time                       Type              Description
2018-04-12T17:50:54-05:00  Not Restarting    Exceeded allowed attempts 3 in interval 30m0s and mode is "fail"
2018-04-12T17:50:54-05:00  Killed            Task successfully killed
2018-04-12T17:50:54-05:00  Killing           Sent interrupt. Waiting 5s before force killing
2018-04-12T17:50:54-05:00  Restart Signaled  healthcheck: check "service: \"demo-service-test\" check" unhealthy
2018-04-12T17:50:32-05:00  Started           Task started by client
2018-04-12T17:50:15-05:00  Restarting        Task restarting in 16.887291122s
2018-04-12T17:50:15-05:00  Killed            Task successfully killed
2018-04-12T17:50:15-05:00  Killing           Sent interrupt. Waiting 5s before force killing
2018-04-12T17:50:15-05:00  Restart Signaled  healthcheck: check "service: \"demo-service-test\" check" unhealthy
2018-04-12T17:49:53-05:00  Started           Task started by client
```

[check restart]: /docs/job-specification/check_restart.html "Nomad check restart Stanza"

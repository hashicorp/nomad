---
layout: "docs"
page_title: "Check Restart Stanza - Operating a Job"
sidebar_current: "docs-operating-a-job-failure-handling-strategies-check-restart"
description: |-
  Nomad can restart service job tasks if they have a failing health check based on
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

[check restart]: /docs/job-specification/check_restart.html "Nomad check restart Stanza"
---
layout: "guides"
page_title: "Handling Failures - Operating a Job"
sidebar_current: "guides-operating-a-job-failure-handling-strategies"
description: |-
  This section describes features in Nomad that automate recovering from failed tasks.
---

# Failure Recovery Strategies

Most applications deployed in Nomad are either long running services or one time batch jobs.
They can fail for various reasons like:

- A temporary error in the service that resolves when its restarted.
- An upstream dependency might not be available, leading to a health check failure.
- Disk, Memory or CPU contention on the node that the application is running on.
- The application uses Docker and the Docker daemon on that node is unresponsive.

Nomad provides configurable options to enable recovering failed tasks to avoid downtime. Nomad will
try to restart a failed task on the node it is running on, and also try to reschedule it on another node.
Please see one of the guides below or use the navigation on the left for details on each option:

1. [Local Restarts](/guides/operating-a-job/failure-handling-strategies/restart.html)
1. [Check Restarts](/guides/operating-a-job/failure-handling-strategies/check-restart.html)
1. [Rescheduling](/guides/operating-a-job/failure-handling-strategies/reschedule.html)

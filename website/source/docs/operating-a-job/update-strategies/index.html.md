---
layout: "docs"
page_title: "Update Strategies - Operating a Job"
sidebar_current: "docs-operating-a-job-updating"
description: |-
  This section describes common patterns for updating already-running jobs
  including rolling upgrades, blue/green deployments, and canary builds. Nomad
  provides built-in support for this functionality.
---

# Update Strategies

Most applications are long-lived and require updates over time. Whether you are
deploying a new version of your web application or upgrading to a new version of
Redis, Nomad has built-in support for rolling, blue/green, and canary updates.
When a job specifies a rolling update, Nomad uses task state and health check
information in order to detect allocation health and minimize or eliminate
downtime. This section and subsections will explore how to do so safely with
Nomad.

Please see one of the guides below or use the navigation on the left:

1. [Rolling Upgrades](/docs/operating-a-job/update-strategies/rolling-upgrades.html)
1. [Blue/Green &amp; Canary Deployments](/docs/operating-a-job/update-strategies/blue-green-and-canary-deployments.html)
1. [Handling Signals](/docs/operating-a-job/update-strategies/handling-signals.html)

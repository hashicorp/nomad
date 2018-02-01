---
layout: "docs"
page_title: "reschedule Stanza - Job Specification"
sidebar_current: "docs-job-specification-reschedule"
description: |-
  The "reschedule" stanza specifies the group's rescheduling strategy upon task failures.
  The reschedule strategy can be configured with number of attempts and a time interval.
  Nomad will attempt to reschedule failed allocations on to another node only after
  any applicable [restarts](docs/job-specification/restart.html) have been tried.
---

# `reschedule` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>job -> **reschedule**</code>
    </td>
    <td>
      <code>job -> group -> **reschedule**</code>
    </td>
  </tr>
</table>

The `reschedule` stanza specifies the group's rescheduling strategy.
It can be configured with number of attempts and a time interval. If
omitted, a failed allocation will not be rescheduled on another node. If specified
at the job level, the configuration will apply to all groups within the job. If multiple
`reschedule` stanzas are specified, they are merged with the group stanza taking the
highest precedence and then the job.

Nomad will attempt to schedule the task on another node if any of its allocation statuses become
"failed". It uses a penalty score to prefer nodes on which the task has not been previously run on.

```hcl
job "docs" {
  group "example" {
    reschedule {
      attempts = 3
      interval    = "15m"
    }
  }
}
```

~> The reschedule stanza does not apply to `system` jobs because they run on every node.

## `reschedule` Parameters

- `attempts` `(int: <varies>)` - Specifies the number of reschedule attempts allowed in the
  configured interval. Defaults vary by job type, see below for more
  information.

- `interval` `(string: <varies>)` - Specifies the duration which begins when the
  first reschedule attempt starts and ensures that only `attempts` number of reschedule happen
  within it. If more than `attempts` number of failures happen with this interval, Nomad will
  not reschedule any more.

Information about reschedule attempts are displayed in the CLI and API for allocations.

### `reschedule` Parameter Defaults

The values for the `reschedule` parameters vary by job type. Here are the
defaults by job type:

- The default batch reschedule policy is:

    ```hcl
    reschedule {
      attempts = 1
      interval = "24h"
    }
    ```

- The default non-batch reschedule policy is:

    ```hcl
    reschedule {
      interval = "1h"
      attempts = 2
    }
    ```

### Rescheduling during deployments

The [update stanza](docs/job-specification/update.html) controls rolling updates and canary deployments. A task
group's reschedule stanza does not take affect during a deployment. For example, if a new version of the job
is rolled out and the deployment failed due to a failing allocation, Nomad will not reschedule it.
---
layout: "docs"
page_title: "job Stanza - Job Specification"
sidebar_current: "docs-job-specification-job"
description: |-
  The "job" stanza is the top-most configuration option in the job
  specification. A job is a declarative specification of tasks that Nomad
  should run.
---

# `job` Stanza

<table class="table table-bordered table-striped">
  <tr>
    <th width="120">Placement</th>
    <td>
      <code>**job**</code>
    </td>
  </tr>
</table>

The `job` stanza is the top-most configuration option in the job specification.
A job is a declarative specification of tasks that Nomad should run. Jobs have a
globally unique name, one or many task groups, which are themselves collections
of one or many tasks.

```hcl
job "docs" {
  constraint {
    # ...
  }

  datacenters = ["us-east-1"]

  group "example" {
    # ...
  }

  meta {
    "my-key" = "my-value"
  }

  parameterized {
    # ...
  }

  periodic {
    # ...
  }

  priority = 100

  region = "north-america"

  task "docs" {
    # ...
  }

  update {
    # ...
  }
}
```

## `job` Parameters

- `all_at_once` `(bool: false)` - Controls if the entire set of tasks in the job
  must be placed atomically or if they can be scheduled incrementally. This
  should only be used for special circumstances.

- `all_at_once` `(bool: false)` - Controls whether the scheduler can make
  partial placements if optimistic scheduling resulted in an oversubscribed
  node. This does not control whether all allocations for the job, where all
  would be the desired count for each task group, must be placed atomically.
  This should only be used for special circumstances.

- `constraint` <code>([Constraint][constraint]: nil)</code> -
  This can be provided multiple times to define additional constraints. See the
  [Nomad constraint reference](/docs/job-specification/constraint.html) for more
  details.

- `datacenters` `(array<string>: <required>)` - A list of datacenters in the region which are eligible
  for task placement. This must be provided, and does not have a default.

- `group` <code>([Group][group]: \<required\>)</code> - Specifies the start of a
  group of tasks. This can be provided multiple times to define additional
  groups. Group names must be unique within the job file.

- `meta` <code>([Meta][]: nil)</code> - Specifies a key-value map that annotates
  with user-defined metadata.

- `parameterized` <code>([Parameterized][parameterized]: nil)</code> - Specifies
  the job as a parameterized job such that it can be dispatched against.

- `periodic` <code>([Periodic][]: nil)</code> - Allows the job to be scheduled
  at fixed times, dates or intervals.

- `priority` `(int: 50)` - Specifies the job priority which is used to
  prioritize scheduling and access to resources. Must be between 1 and 100
  inclusively, with a larger value corresponding to a higher priority.

- `region` `(string: "global")` - The region in which to execute the job.

- `type` `(string: "service")` - Specifies the  [Nomad scheduler][scheduler] to
  use. Nomad provides the `service`, `system` and `batch` schedulers.

- `update` <code>([Update][update]: nil)</code> - Specifies the task's update
  strategy. When omitted, rolling updates are disabled.

- `vault` <code>([Vault][]: nil)</code> - Specifies the set of Vault policies
  required by all tasks in this job.

- `vault_token` `(string: "")` - Specifies the Vault token that proves the
  submitter of the job has access to the specified policies in the
  [`vault`][vault] stanza. This field is only used to transfer the token and is
  not stored after job submission.

    !> It is **strongly discouraged** to place the token as a configuration
    parameter like this, since the token could be checked into source control
    accidentally. Users should set the `VAULT_TOKEN` environment variable when
    running the job instead.

## `job` Examples

The following examples only show the `job` stanzas. Remember that the
`job` stanza is only valid in the placements listed above.

### Docker Container

This example job starts a Docker container which runs as a service. Even though
the type is not specified as "service", that is the default job type.

```hcl
job "docs" {
  datacenters = ["default"]

  group "example" {
    task "server" {
      driver = "docker"
      config {
        image = "hashicorp/http-echo"
        args  = ["-text", "hello"]
      }

      resources {
        memory = 128
      }
    }
  }
}
```

### Batch Job

This example job executes the `uptime` command on 10 Nomad clients in the fleet,
restricting the eligble nodes to Linux machines.

```hcl
job "docs" {
  datacenters = ["default"]

  type = "batch"

  constraint {
    attribute = "${attr.kernel.name}"
    value     = "linux"
  }

  group "example" {
    count = 10
    task "uptime" {
      driver = "exec"
      config {
        command = "uptime"
      }
    }
  }
}
```

### Consuming Secrets

This example shows a job which retrieves secrets from Vault and writes those
secrets to a file on disk, which the application then consumes. Nomad handles
all interactions with Vault.

```hcl
job "docs" {
  datacenters = ["default"]

  group "example" {
    task "cat" {
      driver = "exec"

      config {
        command = "cat"
        args    = ["local/secrets.txt"]
      }

      template {
        data        = "{{ secret \"secret/data\" }}"
        destination = "local/secrets.txt"
      }

      vault {
        policies = ["secret-readonly"]
      }
    }
  }
}
```

When submitting this job, you would run:

```
$ VAULT_TOKEN="..." nomad run example.nomad
```

[constraint]: /docs/job-specification/constraint.html "Nomad constraint Job Specification"
[group]: /docs/job-specification/group.html "Nomad group Job Specification"
[meta]: /docs/job-specification/meta.html "Nomad meta Job Specification"
[parameterized]: /docs/job-specification/parameterized.html "Nomad parameterized Job Specification"
[periodic]: /docs/job-specification/periodic.html "Nomad periodic Job Specification"
[task]: /docs/job-specification/task.html "Nomad task Job Specification"
[update]: /docs/job-specification/update.html "Nomad update Job Specification"
[vault]: /docs/job-specification/vault.html "Nomad vault Job Specification"
[scheduler]: /docs/runtime/schedulers.html "Nomad Scheduler Types"
